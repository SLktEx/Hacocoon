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
	defaultResource func(context.Context, core.Workspace) (core.PersistentResource, error)
	runtime         environmentRuntime
	store           environmentStore
	provider        WorkspaceProvider
	now             func() time.Time
	cleanupTimeout  time.Duration
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

func (s *Service) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	return s.create(ctx, spec, nil)
}

func (s *Service) create(ctx context.Context, spec core.EnvironmentSpec, saved *core.Snapshot) (environment core.Environment, err error) {
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
	unlockEnvironment, err := lockLifecycle(ctx, "environment", name)
	if err != nil {
		return core.Environment{}, err
	}
	defer unlockEnvironment()
	mode, err := normalizeAccessMode(spec.AccessMode)
	if err != nil {
		return core.Environment{}, err
	}
	resources, err := core.ResolveResourceBudget(spec.Resources)
	if err != nil {
		return core.Environment{}, err
	}
	if spec.SkipDefaultResource && spec.PersistentResource != "" {
		return core.Environment{}, core.ErrInvalidArgument
	}
	var persistent core.PersistentResource
	if spec.PersistentResource != "" {
		catalog, ok := s.store.(interface {
			GetPersistentResource(context.Context, string) (core.PersistentResource, error)
		})
		if !ok {
			return core.Environment{}, core.ErrUnsupported
		}
		persistent, err = catalog.GetPersistentResource(ctx, spec.PersistentResource)
		if err != nil {
			return core.Environment{}, err
		}
		if persistent.State != "ready" || !core.ValidPersistentResourceRef(persistent.Ref()) {
			return core.Environment{}, core.ErrRecoveryRequired
		}
	}
	var workspace core.Workspace
	if spec.TemporaryWorkspace != nil {
		if spec.WorkspacePath != "" || !core.ValidTemporaryWorkspace(*spec.TemporaryWorkspace) || mode != core.WorkspaceReadWrite || spec.PersistentResource != "" {
			return core.Environment{}, core.ErrInvalidArgument
		}
		workspace = *spec.TemporaryWorkspace
	} else {
		workspace, err = s.provider.Resolve(ctx, WorkspaceRequest{Path: spec.WorkspacePath})
		if err != nil {
			return core.Environment{}, err
		}
	}
	unlock, err := lockWorkspace(ctx, workspace.ID)
	if err != nil {
		return core.Environment{}, fmt.Errorf("lock workspace: %w", err)
	}
	defer unlock()
	// Resolution may have waited behind explicit Workspace deletion. Never attach
	// a new same-name Workspace using the previous owner's lease or permissions.
	if spec.TemporaryWorkspace == nil {
		current, resolveErr := s.provider.Resolve(ctx, WorkspaceRequest{Path: spec.WorkspacePath})
		if resolveErr != nil {
			return core.Environment{}, resolveErr
		}
		if current != workspace {
			return core.Environment{}, core.ErrCapabilityStale
		}
	}
	if _, err := s.store.GetEnvironment(ctx, name); err == nil {
		return core.Environment{}, fmt.Errorf("environment %q: %w", name, core.ErrAlreadyExists)
	} else if !isNotFound(err) {
		return core.Environment{}, err
	}

	if spec.PersistentResource == "" && !spec.SkipDefaultResource && s.defaultResource != nil {
		persistent, err = s.defaultResource(ctx, workspace)
		if err != nil {
			return core.Environment{}, err
		}
		if persistent != (core.PersistentResource{}) && (persistent.WorkspaceID != workspace.ID || persistent.State != "ready" || !core.ValidPersistentResourceRef(persistent.Ref())) {
			return core.Environment{}, core.ErrRecoveryRequired
		}
	}
	instanceID, identityErr := core.NewEnvironmentInstanceID()
	if identityErr != nil {
		return core.Environment{}, identityErr
	}
	lease := core.WorkspaceLease{
		InstanceID:         instanceID,
		PersistentResource: persistent.Ref(),
		WorkspaceID:        workspace.ID,
		SourcePath:         workspace.Path,
		EnvironmentID:      name,
		AccessMode:         mode,
		Owner:              name,
		State:              core.WorkspaceLeaseAcquiring,
		AcquiredAt:         s.now().UTC(),
	}
	if saved != nil {
		lease.SnapshotSource = saved.ID
		err = s.store.(snapshotCreationCatalog).BeginEnvironmentCreateFromSnapshot(ctx, lease, *saved)
	} else {
		err = s.store.BeginEnvironmentCreate(ctx, lease)
	}
	if err != nil {
		return core.Environment{}, fmt.Errorf("begin environment create: %w", err)
	}

	runtimeSpec := core.EnvironmentRuntimeSpec{
		InstanceID:         instanceID,
		TemporaryWorkspace: spec.TemporaryWorkspace != nil,
		PersistentResource: persistent,
		Name:               name,
		WorkspacePath:      workspace.Path,
		ReadOnly:           mode == core.WorkspaceReadOnly,
		Base:               spec.Base,
		Resources:          resources,
	}
	recorded := false
	record := func(created core.EnvironmentRuntime) error {
		if recorded || strings.TrimSpace(created.Ref) == "" {
			return core.ErrIncompatibleState
		}
		lease.RuntimeRef = created.Ref
		if err := s.store.RecordEnvironmentRuntime(ctx, lease); err != nil {
			return err
		}
		recorded = true
		return nil
	}
	var created core.EnvironmentRuntime
	if saved != nil {
		created, err = s.runtime.(snapshotRuntimeCreator).CreateEnvironmentFromSnapshot(ctx, runtimeSpec, *saved, record)
	} else if provider, ok := s.runtime.(interface {
		CreateEnvironmentWithReceipt(context.Context, core.EnvironmentRuntimeSpec, func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)
	}); ok {
		created, err = provider.CreateEnvironmentWithReceipt(ctx, runtimeSpec, record)
	} else {
		created, err = s.runtime.CreateEnvironment(ctx, runtimeSpec)
	}

	if err != nil {
		if lease.RuntimeRef != "" {
			return core.Environment{}, s.failCreatedEnvironment(ctx, lease, fmt.Errorf("create environment %q: %w", name, err))
		}
		if errors.Is(err, core.ErrRecoveryRequired) {
			lease.State = core.WorkspaceLeaseCleanupRequired
			markErr := s.markEnvironmentRecovery(ctx, lease)
			return core.Environment{}, errors.Join(fmt.Errorf("create environment %q: %w", name, err), markErr, core.ErrRecoveryRequired)
		}
		releaseErr := s.finalizeEnvironmentForCleanup(ctx, name)
		return core.Environment{}, errors.Join(fmt.Errorf("create environment %q: %w", name, err), releaseErr)
	}
	if strings.TrimSpace(created.Ref) == "" {
		lease.State = core.WorkspaceLeaseCleanupRequired
		markErr := s.markEnvironmentRecovery(ctx, lease)
		return core.Environment{}, errors.Join(
			fmt.Errorf("provider created environment %q without a durable runtime reference: %w", name, core.ErrIncompatibleState),
			markErr,
			core.ErrRecoveryRequired,
		)
	}

	// Record provider ownership before performing any further validation or
	// persistence. If the process stops after this point, recovery has an exact
	// runtime reference and the Workspace remains conservatively reserved.
	if recorded && lease.RuntimeRef != created.Ref {
		return core.Environment{}, s.failCreatedEnvironment(ctx, lease, core.ErrCapabilityStale)
	}
	if !recorded {
		if err := record(created); err != nil {
			return core.Environment{}, s.failCreatedEnvironment(ctx, lease, fmt.Errorf("record environment runtime ownership: %w", err))
		}
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
		PersistentResource: persistent.Ref(),
		Name:               name,
		Workspace:          workspace,
		AccessMode:         mode,
		Base:               created.Base,
		Resources:          resources,
		RuntimeRef:         created.Ref,
		CreatedAt:          s.now().UTC(),
	}
	lease.State = core.WorkspaceLeaseActive
	if err := s.store.CommitEnvironmentCreate(ctx, environment, lease); err != nil {
		return core.Environment{}, s.failCreatedEnvironment(ctx, lease, fmt.Errorf("commit ready environment: %w", err))
	}
	return environment, nil
}

func (s *Service) Exec(ctx context.Context, name string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	return s.exec(ctx, name, req, "")
}

// ExecForWorkspace serializes execution with inverse lifecycle operations and
// rejects recycled names before the provider can receive saved project input.
func (s *Service) ExecForWorkspace(ctx context.Context, name string, workspace core.WorkspaceID, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if workspace == "" {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	return s.exec(ctx, name, req, workspace)
}

func (s *Service) exec(ctx context.Context, name string, req core.ExecutionRequest, workspace core.WorkspaceID) (result core.ExecutionResult, err error) {
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
			if workspace == "" {
				logger.ErrorContext(ctx, "environment command failed", append(attrs, "error", err)...)
			} else {
				// Saved input and backend diagnostics may contain project secrets.
				// The setup boundary owns the failure log; do not duplicate it.
				logger.DebugContext(ctx, "project command failed", attrs...)
			}
			return
		}
		logger.InfoContext(ctx, "environment command completed", attrs...)
	}()

	if _, err := validateEnvironmentName(name); err != nil {
		return core.ExecutionResult{}, err
	}
	if len(req.Argv) == 0 || len(req.Stdin) > core.MaxExecutionInputBytes {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	if workspace != "" {
		unlock, lockErr := lockLifecycle(ctx, "environment", name)
		if lockErr != nil {
			return core.ExecutionResult{}, lockErr
		}
		defer unlock()
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if workspace != "" && environment.Workspace.ID != workspace {
		return core.ExecutionResult{}, core.ErrIncompatibleState
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

func (s *Service) Delete(ctx context.Context, name string) error { return s.delete(ctx, name, nil) }

func (s *Service) delete(ctx context.Context, name string, expected *core.Workspace) (err error) {
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
	unlock, err := lockLifecycle(ctx, "environment", name)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.checkSnapshotIdle(ctx, name); err != nil {
		return err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err == nil {
		if expected != nil && environment.Workspace != *expected {
			return core.ErrIncompatibleState
		}
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
	if expected != nil && (lease.WorkspaceID != expected.ID || lease.SourcePath != expected.Path) {
		return core.ErrIncompatibleState
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

// ConfigureDefaultResource installs an optional provider-neutral initializer.
// Configure once at composition time, before serving concurrent requests.
func (s *Service) ConfigureDefaultResource(resolve func(context.Context, core.Workspace) (core.PersistentResource, error)) {
	s.defaultResource = resolve
}
