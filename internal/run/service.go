package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const defaultCleanupTimeout = 30 * time.Second

type Spec struct {
	Base                core.BaseName            `json:"base,omitempty"`
	SkipDefaultResource bool                     `json:"skip_default_resource,omitempty"`
	WorkspacePath       string                   `json:"workspace_path"`
	AccessMode          core.WorkspaceAccessMode `json:"access_mode"`
	Resources           core.ResourceBudget      `json:"resources"`
	Argv                []string                 `json:"argv"`
}

type ExecutionResult struct {
	ExitCode        int    `json:"exit_code"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	StdoutBytes     int64  `json:"stdout_bytes"`
	StderrBytes     int64  `json:"stderr_bytes"`
}

type Result struct {
	Environment string          `json:"environment"`
	Execution   ExecutionResult `json:"execution"`
	CleanedUp   bool            `json:"cleaned_up"`
}

type environmentLifecycle interface {
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	DeleteRun(context.Context, string, string) error
}

type ephemeralRunStore interface {
	ListEphemeralRuns(context.Context) ([]core.EphemeralRun, error)
	PutEphemeralRun(context.Context, core.EphemeralRun) error
	DeleteEphemeralRun(context.Context, string) error
}

type ownershipLockFunc func(string, string, bool) (runOwnershipLock, bool, error)

type Service struct {
	cleanupTemporaryWorkspace func(context.Context, core.Workspace) error
	environments              environmentLifecycle
	runs                      ephemeralRunStore
	lockDir                   string
	newName                   func() (string, error)
	now                       func() time.Time
	cleanupTimeout            time.Duration
	acquireOwnership          ownershipLockFunc
}

func New(environments environmentLifecycle) *Service {
	return &Service{
		environments:     environments,
		newName:          randomEnvironmentName,
		now:              time.Now,
		cleanupTimeout:   defaultCleanupTimeout,
		acquireOwnership: acquireOwnershipLock,
	}
}

func NewWithRecovery(environments environmentLifecycle, runs ephemeralRunStore, lockDir string) *Service {
	service := New(environments)
	service.runs = runs
	service.lockDir = lockDir
	return service
}

func (s *Service) recoveryEnabled() bool {
	return s != nil && s.runs != nil && s.lockDir != "" && s.acquireOwnership != nil
}

func (s *Service) Run(ctx context.Context, spec Spec) (Result, error) {
	if len(spec.Argv) == 0 {
		return Result{}, core.ErrInvalidArgument
	}
	return s.run(ctx, spec, core.PersistentResourceRef{}, func(ctx context.Context, environment core.Environment, _ string) (core.ExecutionResult, error) {
		return s.environments.Exec(ctx, environment.Name, core.ExecutionRequest{WorkingDirectory: "/workspace", Argv: append([]string(nil), spec.Argv...)})
	})
}

// MaintainResource holds the existing ephemeral-run ownership through the whole
// operation and cleanup. The reviewed resource is borrowed, never rebound or
// passed to temporary-resource cleanup as the temporary Workspace's own data.
// Callers must use the Environment's current generation for guarded execution.
func (s *Service) MaintainResource(ctx context.Context, resource core.PersistentResourceRef, operation func(context.Context, core.Environment) error) (Result, error) {
	if !core.ValidPersistentResourceRef(resource) || operation == nil {
		return Result{}, core.ErrInvalidArgument
	}
	if !s.recoveryEnabled() || s.cleanupTemporaryWorkspace == nil {
		return Result{}, core.ErrUnsupported
	}
	// An explicit Store already bypasses default provisioning. Combining it with
	// SkipDefaultResource would contradict the canonical create contract.
	return s.run(ctx, Spec{}, resource, func(ctx context.Context, environment core.Environment, _ string) (core.ExecutionResult, error) {
		if environment.PersistentResource != resource {
			return core.ExecutionResult{}, core.ErrCapabilityStale
		}
		return core.ExecutionResult{}, operation(ctx, environment)
	})
}

func (s *Service) run(ctx context.Context, spec Spec, resource core.PersistentResourceRef, operation func(context.Context, core.Environment, string) (core.ExecutionResult, error)) (Result, error) {
	if s == nil || s.environments == nil || operation == nil {
		return Result{}, core.ErrInvalidArgument
	}
	runCtx, stopSignals := withTerminationSignals(ctx)
	defer stopSignals()
	ctx = runCtx

	if err := s.Reconcile(ctx); err != nil {
		return Result{}, fmt.Errorf("reconcile abandoned ephemeral runs: %w", err)
	}
	name, err := s.newName()
	if err != nil {
		return Result{}, fmt.Errorf("allocate run environment name: %w", err)
	}

	var temporary *core.Workspace
	if spec.WorkspacePath == "" {
		if !s.recoveryEnabled() || s.cleanupTemporaryWorkspace == nil || spec.AccessMode == core.WorkspaceReadOnly {
			return Result{}, core.ErrUnsupported
		}
		work, err := core.NewTemporaryWorkspace()
		if err != nil {
			return Result{}, err
		}
		temporary = &work
	}
	instance, err := core.NewEnvironmentInstanceID()
	if err != nil {
		return Result{}, err
	}
	marker := core.EphemeralRun{InstanceID: instance, TemporaryWorkspace: temporary, EnvironmentID: name, State: core.EphemeralRunCreating, CreatedAt: s.now().UTC()}
	var ownership runOwnershipLock
	if s.recoveryEnabled() {
		var acquired bool
		ownership, acquired, err = s.acquireOwnership(s.lockDir, name, true)
		if err != nil {
			return Result{Environment: name}, fmt.Errorf("claim ephemeral run ownership %q: %w", name, err)
		}
		if !acquired {
			return Result{Environment: name}, fmt.Errorf("ephemeral run identity %q is already owned: %w", name, core.ErrAlreadyExists)
		}
		defer func() { _ = ownership.Release() }()
		if err := s.runs.PutEphemeralRun(ctx, marker); err != nil {
			return Result{Environment: name}, fmt.Errorf("persist ephemeral run marker %q: %w", name, err)
		}
	}

	environment, err := s.environments.Create(ctx, core.EnvironmentSpec{
		EphemeralInstance:   instance,
		PersistentResource:  resource.ID,
		ExpectedResource:    resource,
		TemporaryWorkspace:  temporary,
		Base:                spec.Base,
		SkipDefaultResource: spec.SkipDefaultResource,
		Name:                name,
		WorkspacePath:       spec.WorkspacePath,
		AccessMode:          spec.AccessMode,
		Resources:           spec.Resources,
	})
	if err != nil {
		cause := fmt.Errorf("create ephemeral environment: %w", err)
		if !s.recoveryEnabled() {
			return Result{Environment: name}, cause
		}
		if errors.Is(err, core.ErrRecoveryRequired) {
			// Canonical create still owns uncertain runtime cleanup. Do not
			// attempt temporary-data deletion while its lease may remain.
			return Result{Environment: name}, s.recordCleanup(ctx, marker, cause)
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
		cleanupErr := s.cleanupTemporary(cleanupCtx, temporary)
		cancel()
		return Result{Environment: name}, errors.Join(cause, s.recordCleanup(ctx, marker, cleanupErr))
	}

	// Carry one exact run identity through activation, completion and retry,
	// even in compositions that do not persist ephemeral markers.
	marker.EnvironmentID = environment.Name
	marker.TemporaryWorkspace = temporary
	marker.State = core.EphemeralRunActive
	if s.recoveryEnabled() {
		if err := s.runs.PutEphemeralRun(ctx, marker); err != nil {
			cleaned, cleanupErr := s.cleanupOwnedRun(ctx, marker)
			return Result{Environment: environment.Name, CleanedUp: cleaned}, errors.Join(
				fmt.Errorf("activate ephemeral run marker: %w", err), cleanupErr)
		}
	}

	result := Result{Environment: environment.Name}
	execution, execErr := operation(ctx, environment, instance)
	_, stdoutMarker, stdoutMarkerBytes := host.DecodeCapturedOutput(execution.Stdout)
	_, stderrMarker, stderrMarkerBytes := host.DecodeCapturedOutput(execution.Stderr)
	result.Execution = ExecutionResult{
		ExitCode:        execution.ExitCode,
		Stdout:          execution.Stdout,
		Stderr:          execution.Stderr,
		StdoutTruncated: execution.StdoutTruncated || stdoutMarker,
		StderrTruncated: execution.StderrTruncated || stderrMarker,
		StdoutBytes:     outputByteCount(execution.StdoutBytes, stdoutMarkerBytes),
		StderrBytes:     outputByteCount(execution.StderrBytes, stderrMarkerBytes),
	}
	cleaned, cleanupErr := s.cleanupOwnedRun(ctx, marker)
	result.CleanedUp = cleaned
	return result, errors.Join(execErr, cleanupErr)
}

func outputByteCount(explicit, decoded int64) int64 {
	if explicit > 0 {
		return explicit
	}
	return decoded
}

func randomEnvironmentName() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "run-" + hex.EncodeToString(bytes[:]), nil
}
