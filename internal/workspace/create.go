package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func (s *Service) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	return s.create(ctx, spec, nil, nil)
}

func (s *Service) create(ctx context.Context, spec core.EnvironmentSpec, saved *core.Snapshot, creator runtimeCreation) (environment core.Environment, err error) {
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
	if spec.EphemeralInstance != "" && !core.ValidEnvironmentInstanceID(spec.EphemeralInstance) {
		return core.Environment{}, core.ErrInvalidArgument
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
	if (spec.SkipDefaultResource && spec.PersistentResource != "") || (spec.ExpectedResource != (core.PersistentResourceRef{}) && (!core.ValidPersistentResourceRef(spec.ExpectedResource) || spec.ExpectedResource.ID != spec.PersistentResource)) {
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
	if spec.ExpectedResource != (core.PersistentResourceRef{}) && persistent.Ref() != spec.ExpectedResource {
		return core.Environment{}, core.ErrCapabilityStale
	}
	var workspace core.Workspace
	if spec.TemporaryWorkspace != nil {
		if spec.WorkspacePath != "" || !core.ValidTemporaryWorkspace(*spec.TemporaryWorkspace) || mode != core.WorkspaceReadWrite || (spec.PersistentResource != "" && (spec.ExpectedResource != persistent.Ref() || persistent.SourceOnly)) {
			return core.Environment{}, core.ErrInvalidArgument
		}
		workspace = *spec.TemporaryWorkspace
	} else {
		workspace, err = s.provider.Resolve(ctx, WorkspaceRequest{Path: spec.WorkspacePath})
		if err != nil {
			return core.Environment{}, err
		}
	}
	if spec.ExpectedWorkspace != "" && workspace.ID != spec.ExpectedWorkspace {
		return core.Environment{}, core.ErrCapabilityStale
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
	if spec.EphemeralInstance != "" {
		instanceID = spec.EphemeralInstance
	}
	lease := core.WorkspaceLease{
		Ephemeral:          spec.EphemeralInstance != "",
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
		InstanceID:          instanceID,
		TemporaryWorkspace:  spec.TemporaryWorkspace != nil,
		ResourceMaintenance: spec.TemporaryWorkspace != nil && spec.PersistentResource != "",
		PersistentResource:  persistent,
		Name:                name,
		WorkspacePath:       workspace.Path,
		ReadOnly:            mode == core.WorkspaceReadOnly,
		Base:                spec.Base,
		Resources:           resources,
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
	if creator != nil {
		created, err = creator(ctx, runtimeSpec, record)
	} else if saved != nil {
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
