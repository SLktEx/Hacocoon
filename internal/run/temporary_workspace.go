package run

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// ConfigureTemporaryWorkspace opts into runtime-owned scratch work and its
// optional plugin resource cleanup. Recovery must retain this configuration.
func (s *Service) ConfigureTemporaryWorkspace(cleanup func(context.Context, core.Workspace) error) {
	s.cleanupTemporaryWorkspace = cleanup
}

func (s *Service) cleanupRun(ctx context.Context, marker core.EphemeralRun) error {
	if !core.ValidEnvironmentInstanceID(marker.InstanceID) {
		return core.ErrRecoveryRequired
	}
	if err := s.environments.DeleteRun(ctx, marker.EnvironmentID, marker.InstanceID); err != nil {
		return err
	}
	return s.cleanupTemporary(ctx, marker.TemporaryWorkspace)
}

func (s *Service) cleanupTemporary(ctx context.Context, work *core.Workspace) error {
	if work == nil {
		return nil
	}
	if !core.ValidTemporaryWorkspace(*work) || s.cleanupTemporaryWorkspace == nil {
		return fmt.Errorf("temporary Workspace cleanup unavailable: %w", core.ErrRecoveryRequired)
	}
	return s.cleanupTemporaryWorkspace(ctx, *work)
}
