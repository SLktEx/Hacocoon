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
