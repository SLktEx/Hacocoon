package persistentresource

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// EmptyEnvironmentResource uses the canonical persisted maintenance fence;
// provider paths and volume identities never come from the public caller.
func (s *Service) EmptyEnvironmentResource(ctx context.Context, lease core.WorkspaceLease, area core.EnvironmentAttachment) error {
	if s == nil || s.Store == nil || s.Backend == nil {
		return core.ErrUnsupported
	}
	store, ok := s.Store.(interface {
		BeginEnvironmentResourceClear(context.Context, core.WorkspaceLease, core.PersistentResourceRef) (core.PersistentResource, error)
		CommitEnvironmentResourceClear(context.Context, core.WorkspaceLease, core.PersistentResource) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	backend, ok := s.Backend.(interface {
		EmptyEnvironmentResource(context.Context, core.WorkspaceLease, core.EnvironmentAttachment, core.PersistentResource) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	found := false
	for _, a := range lease.Attachments {
		if a == area {
			found = true
		}
	}
	if !found {
		return core.ErrInvalidArgument
	}
	resource, err := store.BeginEnvironmentResourceClear(ctx, lease, area.Resource)
	if err != nil {
		return err
	}
	if err = backend.EmptyEnvironmentResource(ctx, lease, area, resource); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	if err = store.CommitEnvironmentResourceClear(ctx, lease, resource); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return nil
}
