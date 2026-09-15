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
	unlock, err := s.lockLifecycle(ctx, "environment", name)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.checkLifecycleIdle(ctx, name); err != nil {
		return err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(environment.Workspace.Path, "managed:") || environment.Workspace.ID == "" {
		return fmt.Errorf("snapshots require a managed Workspace: %w", core.ErrUnsupported)
	}
	release, err := s.lockWorkspace(ctx, environment.Workspace.ID)
	if err != nil {
		return err
	}
	defer release()
	lease, err := s.inspectOwnedEnvironmentLease(ctx, environment)
	if err != nil {
		return err
	}
	instance := lease.InstanceID
	verifier := s.runtime.(interface {
		VerifyEnvironmentIdentity(context.Context, string, string) error
	})
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
	if len(lease.Attachments) != 0 {
		if _, err := s.resolveEnvironmentRuntimeResources(ctx, lease); err != nil {
			return err
		}
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
		if err := s.checkLifecycleIdle(ctx, name); err != nil {
			return err
		}
		if err := verifier.VerifyEnvironmentIdentity(ctx, environment.RuntimeRef, instance); err != nil {
			return err
		}
		if err := s.startRuntimeWithLease(ctx, environment, lease); err != nil {
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

func (s *Service) checkLifecycleIdle(ctx context.Context, name string) error {
	if store, ok := s.store.(interface {
		CheckEnvironmentResourcesIdle(context.Context, string) error
	}); ok {
		if err := store.CheckEnvironmentResourcesIdle(ctx, name); err != nil {
			return err
		}
	}

	if store, ok := s.store.(interface {
		CheckSnapshotIdle(context.Context, string) error
	}); ok {
		if err := store.CheckSnapshotIdle(ctx, name); err != nil {
			return err
		}
	}
	return nil
}
