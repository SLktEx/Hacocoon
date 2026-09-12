package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// PublishTemporaryBase keeps inverse lifecycle operations excluded throughout
// publication. It cannot publish an ordinary user's Workspace or a recycled name.
func (s *Service) PublishTemporaryBase(ctx context.Context, name string, work core.Workspace, base core.BaseName) (core.BaseInfo, error) {
	if _, err := validateEnvironmentName(name); err != nil {
		return core.BaseInfo{}, err
	}
	if !core.ValidTemporaryWorkspace(work) {
		return core.BaseInfo{}, core.ErrInvalidArgument
	}
	unlock, err := lockLifecycle(ctx, "environment", name)
	if err != nil {
		return core.BaseInfo{}, err
	}
	defer unlock()
	env, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return core.BaseInfo{}, err
	}
	if env.Workspace != work || env.PersistentResource != (core.PersistentResourceRef{}) {
		return core.BaseInfo{}, core.ErrCapabilityStale
	}
	release, err := lockWorkspace(ctx, work.ID)
	if err != nil {
		return core.BaseInfo{}, err
	}
	defer release()
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil {
		return core.BaseInfo{}, err
	}
	if lease.State != core.WorkspaceLeaseActive || lease.EnvironmentID != name || lease.WorkspaceID != work.ID || lease.SourcePath != work.Path || lease.RuntimeRef != env.RuntimeRef || !core.ValidEnvironmentInstanceID(lease.InstanceID) || lease.PersistentResource != (core.PersistentResourceRef{}) {
		return core.BaseInfo{}, core.ErrRecoveryRequired
	}
	publisher, ok := s.runtime.(interface {
		PublishBase(context.Context, core.Environment, core.WorkspaceLease, core.BaseName) (core.BaseInfo, error)
	})
	if !ok {
		return core.BaseInfo{}, core.ErrUnsupported
	}
	return publisher.PublishBase(ctx, env, lease, base)
}
