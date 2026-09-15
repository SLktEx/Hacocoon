package workspace

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// EmptyEnvironmentResource clears only the exact reviewed attachment while
// holding the same Environment/Workspace lifecycle locks used by collection.
func (s *Service) EmptyEnvironmentResource(ctx context.Context, name string, expected core.EnvironmentAttachment) error {
	if _, err := validateEnvironmentName(name); err != nil {
		return err
	}
	if !core.ValidEnvironmentAttachments([]core.EnvironmentAttachment{expected}) {
		return core.ErrInvalidArgument
	}
	manager, ok := s.environmentResources.(interface {
		EmptyEnvironmentResource(context.Context, core.WorkspaceLease, core.EnvironmentAttachment) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	unlock, err := s.lockLifecycle(ctx, "environment", name)
	if err != nil {
		return err
	}
	defer unlock()
	// The canonical clear transition permits its own interrupted maintenance, but
	// still fences snapshots, copying and every other pending area.
	env, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	release, err := s.lockWorkspace(ctx, env.Workspace.ID)
	if err != nil {
		return err
	}
	defer release()
	lease, err := s.inspectOwnedEnvironmentLease(ctx, env)
	if err != nil {
		return err
	}
	found := false
	for _, area := range lease.Attachments {
		if area == expected {
			found = true
		}
	}
	if !found {
		return core.ErrCapabilityStale
	}
	runtime, ok := s.runtime.(interface {
		InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
	})
	if !ok {
		return core.ErrUnsupported
	}
	status, err := runtime.InspectEnvironment(ctx, env.RuntimeRef)
	if err != nil {
		return err
	}
	if status.Absent || status.State != core.EnvironmentStopped {
		return fmt.Errorf("stop the Environment before emptying cache data: %w", core.ErrIncompatibleState)
	}
	return manager.EmptyEnvironmentResource(ctx, lease, expected)
}
