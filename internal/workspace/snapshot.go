package workspace

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
)

// InspectSnapshotSource checks the canonical aggregate under the lifecycle locks.
// This inspection is not a reservation. CaptureSnapshot uses the locked guard.
func (s *Service) InspectSnapshotSource(ctx context.Context, name string) (core.SnapshotSource, error) {
	var source core.SnapshotSource
	err := s.withSnapshotSource(ctx, name, func(_ context.Context, verified core.SnapshotSource) error { source = verified; return nil })
	return source, err
}

// Snapshot capture must reuse this locked path, not act on an old inspection.
func (s *Service) withSnapshotSource(ctx context.Context, name string, operation func(context.Context, core.SnapshotSource) error) error {
	return s.withSnapshotSourceMode(ctx, name, false, operation)
}

func (s *Service) withSnapshotSourceMode(ctx context.Context, name string, quiesce bool, operation func(context.Context, core.SnapshotSource) error) error {
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
	if err != nil {
		return err
	}
	if !strings.HasPrefix(environment.Workspace.Path, "managed:") || environment.Workspace.ID == "" {
		return fmt.Errorf("snapshots require a managed Workspace: %w", core.ErrUnsupported)
	}
	release, err := lockWorkspace(ctx, environment.Workspace.ID)
	if err != nil {
		return err
	}
	defer release()
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil {
		return fmt.Errorf("snapshot requires durable ownership: %w", core.ErrRecoveryRequired)
	}
	if lease.State != core.WorkspaceLeaseActive || lease.Owner == "" || lease.EnvironmentID != name || lease.RuntimeRef == "" || lease.RuntimeRef != environment.RuntimeRef ||
		lease.WorkspaceID != environment.Workspace.ID || lease.SourcePath != environment.Workspace.Path || lease.AccessMode != environment.AccessMode ||
		lease.PersistentResource != environment.PersistentResource || !core.ValidEnvironmentInstanceID(lease.InstanceID) {
		return core.ErrRecoveryRequired
	}
	if environment.PersistentResource != (core.PersistentResourceRef{}) && !core.ValidPersistentResourceRef(environment.PersistentResource) {
		return core.ErrRecoveryRequired
	}
	identities, ok := s.store.(interface {
		EnvironmentInstance(context.Context, core.Environment) (string, error)
	})
	if !ok {
		return core.ErrUnsupported
	}
	instance, err := identities.EnvironmentInstance(ctx, environment)
	if err != nil {
		return err
	}
	if instance != lease.InstanceID {
		return core.ErrCapabilityStale
	}
	verifier, ok := s.runtime.(interface {
		VerifyEnvironmentIdentity(context.Context, string, string) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	if err := verifier.VerifyEnvironmentIdentity(ctx, environment.RuntimeRef, instance); err != nil {
		return err
	}
	runtime, ok := s.runtime.(interface {
		InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
	})
	if !ok {
		return core.ErrUnsupported
	}
	status, err := runtime.InspectEnvironment(ctx, environment.RuntimeRef)
	if err != nil {
		return err
	}
	wasRunning := status.State == core.EnvironmentRunning
	if status.State != core.EnvironmentStopped && (!quiesce || !wasRunning) {
		return fmt.Errorf("stop the Environment before snapshot: %w", core.ErrIncompatibleState)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	type lifecycleRuntime interface {
		StopEnvironment(context.Context, string) error
		StartEnvironment(context.Context, string) error
	}
	var lifecycle lifecycleRuntime
	if wasRunning {
		var supported bool
		lifecycle, supported = s.runtime.(lifecycleRuntime)
		if !supported {
			return core.ErrUnsupported
		}
		if err := lifecycle.StopEnvironment(ctx, environment.RuntimeRef); err != nil {
			return fmt.Errorf("snapshot stop failed; inspect Environment state: %w", err)
		}
		stopped, err := runtime.InspectEnvironment(ctx, environment.RuntimeRef)
		if err != nil {
			return err
		}
		if stopped.State != core.EnvironmentStopped {
			return fmt.Errorf("snapshot stop unconfirmed: %w", core.ErrIncompatibleState)
		}
		if err := verifier.VerifyEnvironmentIdentity(ctx, environment.RuntimeRef, instance); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := operation(ctx, core.SnapshotSource{Environment: environment, InstanceID: instance}); err != nil {
		if wasRunning {
			return fmt.Errorf("snapshot failed; Environment left stopped: %w", err)
		}
		return err
	}
	if wasRunning {
		if err := s.checkSnapshotIdle(ctx, name); err != nil {
			return err
		}
		if err := verifier.VerifyEnvironmentIdentity(ctx, environment.RuntimeRef, instance); err != nil {
			return err
		}
		if err := lifecycle.StartEnvironment(ctx, environment.RuntimeRef); err != nil {
			return fmt.Errorf("snapshot saved but Environment restart failed: %w", err)
		}
		running, err := runtime.InspectEnvironment(ctx, environment.RuntimeRef)
		if err != nil {
			return err
		}
		if running.State != core.EnvironmentRunning {
			return fmt.Errorf("snapshot saved but Environment restart unconfirmed: %w", core.ErrIncompatibleState)
		}
	}
	return nil
}

func (s *Service) checkSnapshotIdle(ctx context.Context, name string) error {
	if store, ok := s.store.(interface {
		CheckSnapshotIdle(context.Context, string) error
	}); ok {
		return store.CheckSnapshotIdle(ctx, name)
	}
	return nil
}
