package workspace

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Stop retains the Environment aggregate and its lease. A stopped runtime still
// owns its Workspace; stopping never authorizes deletion or lease release.
func (s *Service) Stop(ctx context.Context, name string) error {
	if _, err := validateEnvironmentName(name); err != nil {
		return err
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	runtime, ok := s.runtime.(interface {
		StopEnvironment(context.Context, string) error
	})
	if !ok {
		return fmt.Errorf("Environment stop: %w", core.ErrUnsupported)
	}
	return runtime.StopEnvironment(ctx, environment.RuntimeRef)
}
