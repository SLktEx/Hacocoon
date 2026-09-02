package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const defaultCleanupTimeout = 30 * time.Second

var environmentNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)

type environmentRuntime interface {
	CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error)
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	ShellEnvironment(context.Context, string) error
	DeleteEnvironment(context.Context, string) error
}

// environmentStore exposes lifecycle transitions rather than independent
// Environment/Workspace-lease mutations. The production store implements each
// transition atomically, so callers cannot publish an active lease before its
// ready Environment metadata, or remove one side of the aggregate while the
// other remains durable.
type environmentStore interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error)
	BeginEnvironmentCreate(context.Context, core.WorkspaceLease) error
	RecordEnvironmentRuntime(context.Context, core.WorkspaceLease) error
	CommitEnvironmentCreate(context.Context, core.Environment, core.WorkspaceLease) error
	MarkEnvironmentRecoveryRequired(context.Context, core.WorkspaceLease) error
	FinalizeEnvironmentDelete(context.Context, string) error
}

type Service struct {
	runtime        environmentRuntime
	store          environmentStore
	provider       WorkspaceProvider
	now            func() time.Time
	cleanupTimeout time.Duration
}

func New(runtime environmentRuntime, store environmentStore) *Service {
	return NewWithProvider(runtime, store, NewExternalPathWorkspace())
}

func NewWithProvider(runtime environmentRuntime, store environmentStore, provider WorkspaceProvider) *Service {
	return &Service{
		runtime:        runtime,
		store:          store,
		provider:       provider,
		now:            time.Now,
		cleanupTimeout: defaultCleanupTimeout,
	}
}

func (s *Service) Create(ctx context.Context, spec core.EnvironmentSpec) (environment core.Environment, err error) {
	started := time.Now()
	ctx = logging.With(ctx, "operation", "create_environment", "environment_id", spec.Name)
	logger := logging.FromContext(ctx).With("component", "core")
	logger.InfoContext(ctx, "creating environment")
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "environment creation failed",
				"duration_ms", time.Since(started).Milliseconds(),
				"error", err,
			)
			return
		}
		logger.InfoContext(ctx, "environment created",
			"duration_ms", time.Since(started).Milliseconds(),
			"runtime_ref", environment.RuntimeRef,
		)
	}()

	name, err := validateEnvironmentName(spec.Name)
	if err != nil {
		return core.Environment{}, err
	}
	mode, err := normalizeAccessMode(spec.AccessMode)
	if err != nil {
		return core.Environment{}, err
	}
	resources, err := core.ResolveResourceBudget(spec.Resources)
	if err != nil {
		return core.Environment{}, err
	}
	workspace, err := s.provider.Resolve(ctx, WorkspaceRequest{Path: spec.WorkspacePath})
	if err != nil {
		return core.Environment{}, err
	}
	unlock, err := lockWorkspace(ctx, workspace.ID)
	if err != nil {
		return core.Environment{}, fmt.Errorf("lock workspace: %w", err)
	}
	defer unlock()
	if _, err := s.store.GetEnvironment(ctx, name); err == nil {
		return core.Environment{}, fmt.Errorf("environment %q: %w", name, core.ErrAlreadyExists)
	} else if !isNotFound(err) {
		return core.Environment{}, err
	}

	lease := core.WorkspaceLease{
		WorkspaceID:   workspace.ID,
		SourcePath:    workspace.Path,
		EnvironmentID: name,
		AccessMode:    mode,
		Owner:         name,
		State:         core.WorkspaceLeaseAcquiring,
		AcquiredAt:    s.now().UTC(),
	}
	if err := s.store.BeginEnvironmentCreate(ctx, lease); err != nil {
		return core.Environment{}, fmt.Errorf("begin environment create: %w", err)
	}

	created, createErr := s.runtime.CreateEnvironment(ctx, core.EnvironmentRuntimeSpec{
		Name:          name,
		WorkspacePath: workspace.Path,
		ReadOnly:      mode == core.WorkspaceReadOnly,
		Base:          spec.Base,
		Resources:     resources,
	})

	// A non-empty provider reference is ownership evidence even when the
	// provider also returns an error. Persist that identity before interpreting
	// the error or attempting cleanup so a crash cannot turn a known runtime
	// into an anonymous resource that later retries might forget or duplicate.
	if ref := strings.TrimSpace(created.Ref); ref != "" {
		lease.RuntimeRef = ref
		lease.State = core.WorkspaceLeaseAcquiring
		if recordErr := s.store.RecordEnvironmentRuntime(ctx, lease); recordErr != nil {
			cause := fmt.Errorf("record environment runtime ownership: %w", recordErr)
			if createErr != nil {
				cause = errors.Join(fmt.Errorf("create environment %q: %w", name, createErr), cause)
			}
			return core.Environment{}, s.failCreatedEnvironment(ctx, lease, cause)
		}
		created.Ref = ref
	}

	if createErr != nil {
		if lease.RuntimeRef != "" {
			if errors.Is(createErr, core.ErrRecoveryRequired) {
				lease.State = core.WorkspaceLeaseCleanupRequired
				markErr := s.markEnvironmentRecovery(ctx, lease)
				return core.Environment{}, errors.Join(fmt.Errorf("create environment %q: %w", name, createErr), markErr, core.ErrRecoveryRequired)
			}
			return core.Environment{}, s.failCreatedEnvironment(ctx, lease, fmt.Errorf("create environment %q: %w", name, createErr))
		}
		if errors.Is(createErr, core.ErrRecoveryRequired) {
			lease.State = core.WorkspaceLeaseCleanupRequired
			markErr := s.markEnvironmentRecovery(ctx, lease)
			return core.Environment{}, errors.Join(fmt.Errorf("create environment %q: %w", name, createErr), markErr, core.ErrRecoveryRequired)
		}
		releaseErr := s.finalizeEnvironmentForCleanup(ctx, name)
		return core.Environment{}, errors.Join(fmt.Errorf("create environment %q: %w", name, createErr), releaseErr)
	}
	if lease.RuntimeRef == "" {
		lease.State = core.WorkspaceLeaseCleanupRequired
		markErr := s.markEnvironmentRecovery(ctx, lease)
		return core.Environment{}, errors.Join(
			fmt.Errorf("provider created environment %q without a durable runtime reference: %w", name, core.ErrIncompatibleState),
			markErr,
			core.ErrRecoveryRequired,
		)
	}

	if created.Resources == (core.ResourceBudget{}) && !core.ResourceBudgetHasFinite(resources) {
		created.Resources = resources
	}
	if created.Resources != resources {
		return core.Environment{}, s.failCreatedEnvironment(
			ctx,
			lease,
			fmt.Errorf("provider returned resource budget different from requested effective budget: %w", core.ErrIncompatibleState),
		)
	}

	environment = core.Environment{
		Name:       name,
		Workspace:  workspace,
		AccessMode: mode,
		Base:       created.Base,
		Resources:  resources,
		RuntimeRef: created.Ref,
		CreatedAt:  s.now().UTC(),
	}
	lease.State = core.WorkspaceLeaseActive
	if err := s.store.CommitEnvironmentCreate(ctx, environment, lease); err != nil {
		return core.Environment{}, s.failCreatedEnvironment(ctx, lease, fmt.Errorf("commit ready environment: %w", err))
	}
	return environment, nil
}

func (s *Service) Exec(ctx context.Context, name string, req core.ExecutionRequest) (result core.ExecutionResult, err error) {
	started := time.Now()
	ctx = logging.With(ctx, "operation", "exec_environment", "environment_id", name)
	logger := logging.FromContext(ctx).With("component", "core")
	logger.InfoContext(ctx, "executing environment command")
	defer func() {
		attrs := []any{
			"duration_ms", time.Since(started).Milliseconds(),
			"exit_code", result.ExitCode,
		}
		if err != nil {
			logger.ErrorContext(ctx, "environment command failed", append(attrs, "error", err)...)
			return
		}
		logger.InfoContext(ctx, "environment command completed", attrs...)
	}()

	if _, err := validateEnvironmentName(name); err != nil {
		return core.ExecutionResult{}, err
	}
	if len(req.Argv) == 0 {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	return s.runtime.ExecEnvironment(ctx, environment.RuntimeRef, req)
}

func (s *Service) Shell(ctx context.Context, name string) (err error) {
	started := time.Now()
	ctx = logging.With(ctx, "operation", "shell_environment", "environment_id", name)
	logger := logging.FromContext(ctx).With("component", "core")
	logger.InfoContext(ctx, "opening environment shell")
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "environment shell failed",
				"duration_ms", time.Since(started).Milliseconds(),
				"error", err,
			)
			return
		}
		logger.InfoContext(ctx, "environment shell closed", "duration_ms", time.Since(started).Milliseconds())
	}()

	if _, err := validateEnvironmentName(name); err != nil {
		return err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	return s.runtime.ShellEnvironment(ctx, environment.RuntimeRef)
}

func (s *Service) Delete(ctx context.Context, name string) (err error) {
	started := time.Now()
	ctx = logging.With(ctx, "operation", "delete_environment", "environment_id", name)
	logger := logging.FromContext(ctx).With("component", "core")
	logger.InfoContext(ctx, "deleting environment")
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "environment deletion failed",
				"duration_ms", time.Since(started).Milliseconds(),
				"error", err,
			)
			return
		}
		logger.InfoContext(ctx, "environment deleted", "duration_ms", time.Since(started).Milliseconds())
	}()

	if _, err := validateEnvironmentName(name); err != nil {
		return err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err == nil {
		if err := s.runtime.DeleteEnvironment(ctx, environment.RuntimeRef); err != nil && !isNotFound(err) {
			return fmt.Errorf("delete runtime %q: %w", environment.RuntimeRef, err)
		}
		if err := s.store.FinalizeEnvironmentDelete(ctx, name); err != nil {
			return fmt.Errorf("finalize environment deletion %q: %w", name, err)
		}
		return nil
	}
	if !isNotFound(err) {
		return err
	}

	lease, leaseErr := s.store.GetWorkspaceLease(ctx, name)
	if isNotFound(leaseErr) {
		return nil
	}
	if leaseErr != nil {
		return leaseErr
	}
	if lease.RuntimeRef == "" {
		return fmt.Errorf("workspace lease for %q has no runtime reference; refusing to reclaim without proof: %w", name, core.ErrRecoveryRequired)
	}
	if err := s.runtime.DeleteEnvironment(ctx, lease.RuntimeRef); err != nil && !isNotFound(err) {
		return fmt.Errorf("recover runtime %q: %w", lease.RuntimeRef, err)
	}
	if err := s.store.FinalizeEnvironmentDelete(ctx, name); err != nil {
		return fmt.Errorf("finalize recovered environment deletion %q: %w", name, err)
	}
	return nil
}

// failCreatedEnvironment is the single cleanup policy for a provider runtime
// that has already been created and durably associated with the Environment.
// Successful provider cleanup permits the lifecycle reservation to be removed;
// uncertain cleanup preserves ownership and explicitly marks recovery required.
func (s *Service) failCreatedEnvironment(parent context.Context, lease core.WorkspaceLease, cause error) error {
	cleanupErr := s.deleteRuntimeForCleanup(parent, lease.RuntimeRef)
	if cleanupErr == nil {
		return errors.Join(cause, s.finalizeEnvironmentForCleanup(parent, lease.EnvironmentID))
	}
	lease.State = core.WorkspaceLeaseCleanupRequired
	return errors.Join(cause, cleanupErr, s.markEnvironmentRecovery(parent, lease), core.ErrRecoveryRequired)
}

func (s *Service) deleteRuntimeForCleanup(parent context.Context, ref string) error {
	cleanupCtx, cancel := s.newCleanupContext(parent)
	defer cancel()
	if err := s.runtime.DeleteEnvironment(cleanupCtx, ref); err != nil && !isNotFound(err) {
		return fmt.Errorf("cleanup runtime %q: %w", ref, err)
	}
	return nil
}

func (s *Service) finalizeEnvironmentForCleanup(parent context.Context, environmentID string) error {
	cleanupCtx, cancel := s.newCleanupContext(parent)
	defer cancel()
	return s.store.FinalizeEnvironmentDelete(cleanupCtx, environmentID)
}

func (s *Service) markEnvironmentRecovery(parent context.Context, lease core.WorkspaceLease) error {
	cleanupCtx, cancel := s.newCleanupContext(parent)
	defer cancel()
	return s.store.MarkEnvironmentRecoveryRequired(cleanupCtx, lease)
}

func (s *Service) newCleanupContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), s.cleanupTimeout)
}

func validateEnvironmentName(name string) (string, error) {
	if !environmentNamePattern.MatchString(name) {
		return "", fmt.Errorf("environment name %q must use lowercase letters, digits, and internal hyphens: %w", name, core.ErrInvalidArgument)
	}
	return name, nil
}

func normalizeAccessMode(mode core.WorkspaceAccessMode) (core.WorkspaceAccessMode, error) {
	if mode == "" {
		return core.WorkspaceReadWrite, nil
	}
	switch mode {
	case core.WorkspaceReadOnly, core.WorkspaceReadWrite:
		return mode, nil
	default:
		return "", fmt.Errorf("workspace access mode %q: %w", mode, core.ErrInvalidArgument)
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, core.ErrNotFound) || os.IsNotExist(err)
}
