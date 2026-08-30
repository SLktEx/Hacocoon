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
	WorkspacePath string                   `json:"workspace_path"`
	AccessMode    core.WorkspaceAccessMode `json:"access_mode"`
	Resources     core.ResourceBudget      `json:"resources"`
	Argv          []string                 `json:"argv"`
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
	Delete(context.Context, string) error
}

type ephemeralRunStore interface {
	ListEphemeralRuns(context.Context) ([]core.EphemeralRun, error)
	PutEphemeralRun(context.Context, core.EphemeralRun) error
	DeleteEphemeralRun(context.Context, string) error
}

type ownershipLockFunc func(string, string, bool) (runOwnershipLock, bool, error)

type Service struct {
	environments     environmentLifecycle
	runs             ephemeralRunStore
	lockDir          string
	newName          func() (string, error)
	now              func() time.Time
	cleanupTimeout   time.Duration
	acquireOwnership ownershipLockFunc
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

// Reconcile removes only Environments with an explicit durable ephemeral-run
// marker whose process ownership lock is no longer held. A run-* name alone is
// never treated as proof that an Environment is safe to delete.
func (s *Service) Reconcile(ctx context.Context) error {
	if s == nil || s.environments == nil {
		return core.ErrInvalidArgument
	}
	if !s.recoveryEnabled() {
		return nil
	}
	runs, err := s.runs.ListEphemeralRuns(ctx)
	if err != nil {
		return fmt.Errorf("list ephemeral runs for recovery: %w", err)
	}
	var recoveryErrs []error
	for _, run := range runs {
		select {
		case <-ctx.Done():
			return errors.Join(append(recoveryErrs, ctx.Err())...)
		default:
		}
		if run.EnvironmentID == "" {
			recoveryErrs = append(recoveryErrs, fmt.Errorf("ephemeral run marker has no environment identity: %w", core.ErrRecoveryRequired))
			continue
		}
		ownership, acquired, lockErr := s.acquireOwnership(s.lockDir, run.EnvironmentID, true)
		if lockErr != nil {
			recoveryErrs = append(recoveryErrs, fmt.Errorf("probe ownership for ephemeral run %q: %w", run.EnvironmentID, lockErr))
			continue
		}
		if !acquired {
			// Another live Hacocoon process still owns this run.
			continue
		}

		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
		deleteErr := s.environments.Delete(cleanupCtx, run.EnvironmentID)
		cancel()
		if deleteErr == nil {
			if markerErr := s.runs.DeleteEphemeralRun(context.WithoutCancel(ctx), run.EnvironmentID); markerErr != nil {
				recoveryErrs = append(recoveryErrs, fmt.Errorf("remove recovered ephemeral run marker %q: %w", run.EnvironmentID, markerErr))
			}
		} else {
			run.State = core.EphemeralRunCleanupRequired
			markErr := s.runs.PutEphemeralRun(context.WithoutCancel(ctx), run)
			recoveryErrs = append(recoveryErrs, errors.Join(
				fmt.Errorf("recover abandoned ephemeral run %q: %w", run.EnvironmentID, deleteErr),
				markErr,
				core.ErrRecoveryRequired,
			))
		}
		if releaseErr := ownership.Release(); releaseErr != nil {
			recoveryErrs = append(recoveryErrs, fmt.Errorf("release ownership probe for %q: %w", run.EnvironmentID, releaseErr))
		}
	}
	return errors.Join(recoveryErrs...)
}

func (s *Service) Run(ctx context.Context, spec Spec) (Result, error) {
	if s == nil || s.environments == nil || spec.WorkspacePath == "" || len(spec.Argv) == 0 {
		return Result{}, core.ErrInvalidArgument
	}
	if err := s.Reconcile(ctx); err != nil {
		return Result{}, fmt.Errorf("reconcile abandoned ephemeral runs: %w", err)
	}
	name, err := s.newName()
	if err != nil {
		return Result{}, fmt.Errorf("allocate run environment name: %w", err)
	}

	var marker core.EphemeralRun
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
		marker = core.EphemeralRun{
			EnvironmentID: name,
			State:         core.EphemeralRunCreating,
			CreatedAt:     s.now().UTC(),
		}
		if err := s.runs.PutEphemeralRun(ctx, marker); err != nil {
			return Result{Environment: name}, fmt.Errorf("persist ephemeral run marker %q: %w", name, err)
		}
	}

	environment, err := s.environments.Create(ctx, core.EnvironmentSpec{
		Name:          name,
		WorkspacePath: spec.WorkspacePath,
		AccessMode:    spec.AccessMode,
		Resources:     spec.Resources,
	})
	if err != nil {
		if s.recoveryEnabled() {
			if errors.Is(err, core.ErrRecoveryRequired) {
				marker.State = core.EphemeralRunCleanupRequired
				markErr := s.runs.PutEphemeralRun(context.WithoutCancel(ctx), marker)
				return Result{Environment: name}, errors.Join(fmt.Errorf("create ephemeral environment: %w", err), markErr)
			}
			markerErr := s.runs.DeleteEphemeralRun(context.WithoutCancel(ctx), name)
			return Result{Environment: name}, errors.Join(fmt.Errorf("create ephemeral environment: %w", err), markerErr)
		}
		return Result{Environment: name}, fmt.Errorf("create ephemeral environment: %w", err)
	}

	if s.recoveryEnabled() {
		marker.EnvironmentID = environment.Name
		marker.State = core.EphemeralRunActive
		if err := s.runs.PutEphemeralRun(ctx, marker); err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
			cleanupErr := s.environments.Delete(cleanupCtx, environment.Name)
			cancel()
			if cleanupErr == nil {
				markerErr := s.runs.DeleteEphemeralRun(context.WithoutCancel(ctx), environment.Name)
				return Result{Environment: environment.Name, CleanedUp: true}, errors.Join(fmt.Errorf("activate ephemeral run marker: %w", err), markerErr)
			}
			marker.State = core.EphemeralRunCleanupRequired
			markErr := s.runs.PutEphemeralRun(context.WithoutCancel(ctx), marker)
			return Result{Environment: environment.Name}, errors.Join(
				fmt.Errorf("activate ephemeral run marker: %w", err),
				fmt.Errorf("cleanup ephemeral environment %q: %w", environment.Name, cleanupErr),
				markErr,
				core.ErrRecoveryRequired,
			)
		}
	}

	result := Result{Environment: environment.Name}
	execution, execErr := s.environments.Exec(ctx, environment.Name, core.ExecutionRequest{Argv: append([]string(nil), spec.Argv...)})
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
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
	cleanupErr := s.environments.Delete(cleanupCtx, environment.Name)
	cancel()
	result.CleanedUp = cleanupErr == nil

	var markerErr error
	if s.recoveryEnabled() {
		if cleanupErr == nil {
			markerErr = s.runs.DeleteEphemeralRun(context.WithoutCancel(ctx), environment.Name)
		} else {
			marker.State = core.EphemeralRunCleanupRequired
			markerErr = s.runs.PutEphemeralRun(context.WithoutCancel(ctx), marker)
		}
	}

	if execErr != nil && cleanupErr != nil {
		return result, errors.Join(execErr, fmt.Errorf("cleanup ephemeral environment %q: %w", environment.Name, cleanupErr), markerErr)
	}
	if execErr != nil {
		return result, errors.Join(execErr, markerErr)
	}
	if cleanupErr != nil {
		return result, errors.Join(fmt.Errorf("cleanup ephemeral environment %q: %w", environment.Name, cleanupErr), markerErr)
	}
	if markerErr != nil {
		return result, fmt.Errorf("remove completed ephemeral run marker %q: %w", environment.Name, markerErr)
	}
	return result, nil
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
