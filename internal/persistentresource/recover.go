package persistentresource

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// RecoverCopy publishes only a copy with a durable positive completion receipt.
// Unknown provider completion never becomes success from object existence alone.
func (s *Service) RecoverCopy(ctx context.Context, id string) (core.PersistentResource, error) {
	target, err := s.Store.GetPersistentResource(ctx, id)
	if err != nil {
		return target, err
	}
	if target.State == "ready" && target.CopySource == (core.PersistentResourceRef{}) && !target.CopyCompleted {
		// Another recovery may have published and attached it already. This
		// operation is finished; do not reject that legitimate new attachment.
		return target, nil
	}
	if target.State != "creating" || !target.CopyCompleted || target.CopySource == (core.PersistentResourceRef{}) {
		return target, core.ErrRecoveryRequired
	}
	source, err := s.Store.GetPersistentResource(ctx, target.CopySource.ID)
	if err != nil {
		return target, err
	}
	if source.Ref() != target.CopySource || source.State != "ready" || source.Kind != target.Kind {
		return target, core.ErrRecoveryRequired
	}
	if err := s.Backend.Verify(ctx, target); err != nil {
		return target, err
	}
	if backend, ok := s.Backend.(interface {
		RecoverCompletedCopy(context.Context, core.PersistentResource, core.PersistentResource) error
	}); ok {
		if err := backend.RecoverCompletedCopy(ctx, source, target); err != nil {
			return target, err
		}
	}
	if err := s.Store.CommitPersistentResourceCreate(ctx, target); err != nil {
		// Another verified recovery may already have committed this exact identity.
		current, readErr := s.Store.GetPersistentResource(ctx, id)
		expected := target
		expected.State = "ready"
		expected.CopySource = core.PersistentResourceRef{}
		expected.CopyCompleted = false
		if readErr == nil && current == expected {
			return current, nil
		}
		return target, err
	}
	target.State = "ready"
	target.CopySource = core.PersistentResourceRef{}
	target.CopyCompleted = false
	return target, nil
}
