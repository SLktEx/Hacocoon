package workspace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func (s *Service) Delete(ctx context.Context, name string) error { return s.delete(ctx, name, nil, "") }

func (s *Service) delete(ctx context.Context, name string, expected *core.Workspace, instance string) (err error) {
	return s.deleteWithPolicy(ctx, name, expected, instance, false, false)
}

// DeleteUser enforces the public lifecycle policy under the same lock as start.
// Internal failed-create/temporary cleanup continues to use exact-owner deletion.
func (s *Service) DeleteUser(ctx context.Context, name string, force bool) error {
	return s.deleteWithPolicy(ctx, name, nil, "", true, force)
}

func (s *Service) deleteWithPolicy(ctx context.Context, name string, expected *core.Workspace, instance string, user, force bool) (err error) {
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
	unlock, err := s.lockLifecycle(ctx, "environment", name)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.checkLifecycleIdle(ctx, name); err != nil {
		return err
	}
	if instance != "" {
		lease, leaseErr := s.store.GetWorkspaceLease(ctx, name)
		if leaseErr == nil {
			if !lease.Ephemeral || lease.InstanceID != instance {
				return core.ErrCapabilityStale
			}
		} else if !isNotFound(leaseErr) {
			return leaseErr
		} else if _, envErr := s.store.GetEnvironment(ctx, name); !isNotFound(envErr) {
			return errors.Join(core.ErrRecoveryRequired, envErr)
		}
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err == nil {
		if expected != nil && environment.Workspace != *expected {
			return core.ErrIncompatibleState
		}
		if user {
			runtime, ok := s.runtime.(interface {
				InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
				StopEnvironment(context.Context, string) error
			})
			if !ok {
				return core.ErrUnsupported
			}
			status, err := runtime.InspectEnvironment(ctx, environment.RuntimeRef)
			if err != nil {
				return err
			}
			if status.State == core.EnvironmentRunning {
				if !force {
					return fmt.Errorf("running Environment; stop it or use -f: %w", core.ErrStorageBusy)
				}
				if err := runtime.StopEnvironment(ctx, environment.RuntimeRef); err != nil {
					return err
				}
			} else if status.State != core.EnvironmentStopped {
				return core.ErrIncompatibleState
			}
		}
		if err := s.deleteAndFinalize(ctx, name, environment.RuntimeRef); err != nil {
			return err
		}
		return s.finishOwnedWorkspaceCleanup(ctx, name)

	}
	if !isNotFound(err) {
		return err
	}

	lease, leaseErr := s.store.GetWorkspaceLease(ctx, name)
	if isNotFound(leaseErr) {
		return s.finishOwnedWorkspaceCleanup(ctx, name)
	}
	if leaseErr != nil {
		return leaseErr
	}
	if expected != nil && (lease.WorkspaceID != expected.ID || lease.SourcePath != expected.Path) {
		return core.ErrIncompatibleState
	}
	if lease.RuntimeAbsent {
		return s.finalizeAbsentEnvironment(ctx, name)
	}
	if lease.RuntimeRef == "" {
		return fmt.Errorf("workspace lease for %q has no runtime reference; refusing to reclaim without proof: %w", name, core.ErrRecoveryRequired)
	}
	return s.deleteAndFinalize(ctx, name, lease.RuntimeRef)
}

func (s *Service) finishOwnedWorkspaceCleanup(ctx context.Context, name string) error {
	catalog, ok := s.store.(interface {
		OwnedWorkspaceCleanup(context.Context, string) (core.Workspace, error)
		FinalizeOwnedWorkspaceCleanup(context.Context, string, core.Workspace) error
	})
	if !ok {
		return nil
	}
	work, err := catalog.OwnedWorkspaceCleanup(ctx, name)
	if errors.Is(err, core.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.DeleteOwnedWorkspace(ctx, work); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return catalog.FinalizeOwnedWorkspaceCleanup(ctx, name, work)
}
