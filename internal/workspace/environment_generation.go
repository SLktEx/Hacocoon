package workspace

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// CollectEnvironmentResource uses human area names but resolves ownership and
// provenance from the current lease. No guest-supplied path or source is read.
func (s *Service) CollectEnvironmentResource(ctx context.Context, name, key string) (result core.ResourceGenerationPublication, err error) {
	if _, err = validateEnvironmentName(name); err != nil {
		return result, err
	}
	publisher, ok := s.environmentResources.(interface {
		PublishEnvironmentGeneration(context.Context, core.WorkspaceLease, core.EnvironmentAttachment) (core.ResourceGenerationPublication, error)
	})
	if !ok {
		return result, core.ErrUnsupported
	}
	unlock, err := s.lockLifecycle(ctx, "environment", name)
	if err != nil {
		return result, err
	}
	defer unlock()
	if err = s.checkLifecycleIdle(ctx, name); err != nil {
		return result, err
	}
	env, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return result, err
	}
	release, err := s.lockWorkspace(ctx, env.Workspace.ID)
	if err != nil {
		return result, err
	}
	defer release()
	lease, err := s.inspectOwnedEnvironmentLease(ctx, env)
	if err != nil {
		return result, err
	}
	runtime, ok := s.runtime.(interface {
		InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
	})
	if !ok {
		return result, core.ErrUnsupported
	}
	status, err := runtime.InspectEnvironment(ctx, env.RuntimeRef)
	if err != nil {
		return result, err
	}
	if status.Absent || status.State != core.EnvironmentStopped {
		return result, fmt.Errorf("stop the Environment before collecting cache data: %w", core.ErrIncompatibleState)
	}
	for _, area := range lease.Attachments {
		if area.Key == key {
			return publisher.PublishEnvironmentGeneration(ctx, lease, area)
		}
	}
	return result, core.ErrNotFound
}

// Shared by stopped snapshot and data publication while holding both locks.
func (s *Service) inspectOwnedEnvironmentLease(ctx context.Context, env core.Environment) (core.WorkspaceLease, error) {
	lease, err := s.store.GetWorkspaceLease(ctx, env.Name)
	if err != nil {
		return lease, fmt.Errorf("operation requires durable ownership: %w", core.ErrRecoveryRequired)
	}
	if lease.EnvironmentID != env.Name || !lease.MatchesEnvironment(env) || lease.Owner == "" || !core.ValidEnvironmentInstanceID(lease.InstanceID) {
		return lease, core.ErrRecoveryRequired
	}
	if env.PersistentResource != (core.PersistentResourceRef{}) && !core.ValidPersistentResourceRef(env.PersistentResource) {
		return lease, core.ErrRecoveryRequired
	}
	identities, ok := s.store.(interface {
		EnvironmentInstance(context.Context, core.Environment) (string, error)
	})
	if !ok {
		return lease, core.ErrUnsupported
	}
	instance, err := identities.EnvironmentInstance(ctx, env)
	if err != nil {
		return lease, err
	}
	if instance != lease.InstanceID {
		return lease, core.ErrCapabilityStale
	}
	verifier, ok := s.runtime.(interface {
		VerifyEnvironmentIdentity(context.Context, string, string) error
	})
	if !ok {
		return lease, core.ErrUnsupported
	}
	return lease, verifier.VerifyEnvironmentIdentity(ctx, env.RuntimeRef, instance)
}
