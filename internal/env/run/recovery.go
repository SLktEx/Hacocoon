package run

import (
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
)

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

		if _, cleanupErr := s.cleanupOwnedRun(ctx, run); cleanupErr != nil {
			recoveryErrs = append(recoveryErrs, fmt.Errorf("recover abandoned ephemeral run %q: %w", run.EnvironmentID, cleanupErr))
		}
		if releaseErr := ownership.Release(); releaseErr != nil {
			recoveryErrs = append(recoveryErrs, fmt.Errorf("release ownership probe for %q: %w", run.EnvironmentID, releaseErr))
		}
	}
	return errors.Join(recoveryErrs...)
}
