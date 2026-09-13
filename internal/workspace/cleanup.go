package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

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
	if err := s.runtime.DeleteEnvironment(cleanupCtx, ref); !core.EnvironmentDeletionComplete(err) {
		return fmt.Errorf("cleanup runtime %q: %w", ref, err)
	}
	return nil
}

func (s *Service) finalizeEnvironmentForCleanup(parent context.Context, environmentID string) error {
	cleanupCtx, cancel := s.newCleanupContext(parent)
	defer cancel()
	if err := s.store.FinalizeEnvironmentDelete(cleanupCtx, environmentID); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return nil
}

func (s *Service) markEnvironmentRecovery(parent context.Context, lease core.WorkspaceLease) error {
	cleanupCtx, cancel := s.newCleanupContext(parent)
	defer cancel()
	return s.store.MarkEnvironmentRecoveryRequired(cleanupCtx, lease)
}

func (s *Service) newCleanupContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), s.cleanupTimeout)
}

// deleteAndFinalize is shared by ready, incomplete and temporary Environments.
// A provider success includes its owned network cleanup; absence of only the
// instance is insufficient when another cleanup component failed.
func (s *Service) deleteAndFinalize(ctx context.Context, name, ref string) error {
	if err := s.runtime.DeleteEnvironment(ctx, ref); !core.EnvironmentDeletionComplete(err) {
		cause := fmt.Errorf("delete runtime %q: %w", ref, err)
		lease, leaseErr := s.store.GetWorkspaceLease(ctx, name)
		if leaseErr != nil {
			return errors.Join(cause, leaseErr, core.ErrRecoveryRequired)
		}
		if lease.RuntimeRef != ref {
			return errors.Join(cause, core.ErrCapabilityStale, core.ErrRecoveryRequired)
		}
		lease.State = core.WorkspaceLeaseCleanupRequired
		return errors.Join(cause, s.markEnvironmentRecovery(ctx, lease), core.ErrRecoveryRequired)
	}
	if err := s.store.FinalizeEnvironmentDelete(ctx, name); err != nil {
		return errors.Join(fmt.Errorf("finalize environment deletion %q: %w", name, err), core.ErrRecoveryRequired)
	}
	return nil
}
