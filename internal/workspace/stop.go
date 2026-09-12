package workspace

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Stop retains the Environment aggregate and its lease. A stopped runtime still
// owns its Workspace; stopping never authorizes deletion or lease release.
func (s *Service) Stop(ctx context.Context, name string) error {
	return s.stopForWorkspace(ctx, name, "")
}

func (s *Service) StopForWorkspace(ctx context.Context, name string, work core.WorkspaceID) error {
	if work == "" {
		return core.ErrInvalidArgument
	}
	return s.stopForWorkspace(ctx, name, work)
}
func (s *Service) stopForWorkspace(ctx context.Context, name string, work core.WorkspaceID) error {
	if _, err := validateEnvironmentName(name); err != nil {
		return err
	}
	unlock, err := lockLifecycle(ctx, "environment", name)
	if err != nil {
		return err
	}
	defer unlock()
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	if work != "" && environment.Workspace.ID != work {
		return core.ErrCapabilityStale
	}
	runtime, ok := s.runtime.(interface {
		StopEnvironment(context.Context, string) error
	})
	if !ok {
		return fmt.Errorf("Environment stop: %w", core.ErrUnsupported)
	}
	return runtime.StopEnvironment(ctx, environment.RuntimeRef)
}
