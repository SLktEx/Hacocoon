package workspace

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Start resumes the owned runtime without changing the Environment aggregate.
// A name lock spans observation and the provider call, including inverse operations.
func (s *Service) Start(ctx context.Context, name string) error {
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
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil {
		return fmt.Errorf("resume requires durable lease: %w", core.ErrRecoveryRequired)
	}
	if lease.State != core.WorkspaceLeaseActive || lease.EnvironmentID != name ||
		lease.RuntimeRef == "" || lease.RuntimeRef != environment.RuntimeRef ||
		lease.WorkspaceID != environment.Workspace.ID || lease.SourcePath != environment.Workspace.Path ||
		lease.AccessMode != environment.AccessMode || lease.PersistentResource != environment.PersistentResource {
		return fmt.Errorf("resume ownership does not match: %w", core.ErrRecoveryRequired)
	}
	runtime, ok := s.runtime.(interface {
		StartEnvironment(context.Context, string) error
	})
	if !ok {
		return fmt.Errorf("Environment start: %w", core.ErrUnsupported)
	}
	return runtime.StartEnvironment(ctx, environment.RuntimeRef)
}
