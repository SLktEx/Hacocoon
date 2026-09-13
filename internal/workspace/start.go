package workspace

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Start resumes the owned runtime without changing the Environment aggregate.
// A name lock spans observation and the provider call, including inverse operations.
func (s *Service) Start(ctx context.Context, name string) error {
	return s.start(ctx, name, "")
}

// StartForWorkspace refuses a recycled name before any provider mutation.
func (s *Service) StartForWorkspace(ctx context.Context, name string, expected core.WorkspaceID) error {
	if expected == "" {
		return core.ErrInvalidArgument
	}
	return s.start(ctx, name, expected)
}

func (s *Service) start(ctx context.Context, name string, expected core.WorkspaceID) error {
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
	if err != nil {
		return err
	}
	if expected != "" && environment.Workspace.ID != expected {
		return core.ErrIncompatibleState
	}
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil {
		return fmt.Errorf("resume requires durable lease: %w", core.ErrRecoveryRequired)
	}
	if lease.EnvironmentID != name || !lease.MatchesEnvironment(environment) {
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
