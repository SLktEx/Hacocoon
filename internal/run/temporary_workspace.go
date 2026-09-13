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
	var err error
	if marker.InstanceID != "" {
		if !core.ValidEnvironmentInstanceID(marker.InstanceID) {
			return core.ErrRecoveryRequired
		}
		err = s.environments.DeleteRun(ctx, marker.EnvironmentID, marker.InstanceID)
	} else if marker.TemporaryWorkspace != nil {
		// Legacy migration only: an exact random temporary Workspace still binds
		// cleanup. Never invent a generation for retained legacy runs by name.
		owner, ok := s.environments.(interface {
			DeleteTemporary(context.Context, string, core.Workspace) error
		})
		if !ok || !core.ValidTemporaryWorkspace(*marker.TemporaryWorkspace) {
			return core.ErrRecoveryRequired
		}
		err = owner.DeleteTemporary(ctx, marker.EnvironmentID, *marker.TemporaryWorkspace)
	} else {
		return fmt.Errorf("legacy run lacks a creation identity: %w", core.ErrRecoveryRequired)
	}
	if err != nil {
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
