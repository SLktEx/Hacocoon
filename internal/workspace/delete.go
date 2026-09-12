package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

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
		return s.deleteAndFinalize(ctx, name, environment.RuntimeRef)
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
	return s.deleteAndFinalize(ctx, name, lease.RuntimeRef)
}
