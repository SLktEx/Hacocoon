package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// cleanupOwnedRun is shared by completed runs, failed activation and recovery.
// The canonical Environment owner removes the runtime before optional scratch
// data. Persistent Workspace/Store data is never passed to scratch cleanup.
func (s *Service) cleanupOwnedRun(ctx context.Context, marker core.EphemeralRun) (bool, error) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
	cleanupErr := s.cleanupRun(cleanupCtx, marker)
	cancel()
	return cleanupErr == nil, s.recordCleanup(ctx, marker, cleanupErr)
}

// recordCleanup changes only this run's recovery evidence. It does not infer
// absence or release Environment/Workspace reservations; canonical deletion
// already owns those transitions. Marker writes preserve the existing
// cancellation-independent persistence contract.
func (s *Service) recordCleanup(ctx context.Context, marker core.EphemeralRun, cleanupErr error) error {
	if cleanupErr != nil {
		var markerErr error
		if s.recoveryEnabled() {
			marker.State = core.EphemeralRunCleanupRequired
			markerErr = s.runs.PutEphemeralRun(context.WithoutCancel(ctx), marker)
		}
		return errors.Join(fmt.Errorf("cleanup ephemeral environment %q: %w", marker.EnvironmentID, cleanupErr), markerErr, core.ErrRecoveryRequired)
	}
	if s.recoveryEnabled() {
		if err := s.runs.DeleteEphemeralRun(context.WithoutCancel(ctx), marker.EnvironmentID); err != nil {
			return fmt.Errorf("remove completed ephemeral run marker %q: %w", marker.EnvironmentID, errors.Join(err, core.ErrRecoveryRequired))
		}
	}
	return nil
}
