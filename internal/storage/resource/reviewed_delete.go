package persistentresource

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// DeleteReviewed uses the exact identity shown to the user. Native saved objects
// are checked before transitioning a ready resource, then again at native deletion.
func (s *Service) DeleteReviewed(ctx context.Context, ref core.PersistentResourceRef) error {
	if !core.ValidPersistentResourceRef(ref) {
		return core.ErrInvalidArgument
	}
	catalog, ok := s.Store.(interface {
		BeginPersistentResourceDeleteReviewed(context.Context, core.PersistentResourceRef) (core.PersistentResource, error)
	})
	if !ok {
		return core.ErrUnsupported
	}
	checker, ok := s.Backend.(interface {
		CheckDeletion(context.Context, core.PersistentResource) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	r, err := s.Store.GetPersistentResource(ctx, ref.ID)
	if err != nil {
		return err
	}
	if r.Ref() != ref {
		return core.ErrCapabilityStale
	}
	if r.SourceOnly || (r.State != "ready" && r.State != "deleting") {
		return core.ErrRecoveryRequired
	}
	if err := checker.CheckDeletion(ctx, r); err != nil {
		return err
	}
	owned, err := catalog.BeginPersistentResourceDeleteReviewed(ctx, ref)
	if err != nil {
		return err
	}
	return s.finishDelete(ctx, owned)
}
