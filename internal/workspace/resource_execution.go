package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// ExecForResource binds a reviewed operation to both an Env generation and its
// currently attached persistent resource under the canonical lifecycle lock.
func (s *Service) ExecForResource(ctx context.Context, name, instance string, resource core.PersistentResourceRef, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if _, err := validateEnvironmentName(name); err != nil {
		return core.ExecutionResult{}, err
	}
	if !core.ValidEnvironmentInstanceID(instance) || !core.ValidPersistentResourceRef(resource) {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	unlock, err := lockLifecycle(ctx, "environment", name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	defer unlock()
	env, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	identities, ok := s.store.(interface {
		EnvironmentInstance(context.Context, core.Environment) (string, error)
	})
	if !ok {
		return core.ExecutionResult{}, core.ErrUnsupported
	}
	current, err := identities.EnvironmentInstance(ctx, env)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if current != instance || env.PersistentResource != resource {
		return core.ExecutionResult{}, core.ErrCapabilityStale
	}
	return s.Exec(ctx, name, req)
}
