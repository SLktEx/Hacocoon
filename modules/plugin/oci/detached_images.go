package oci

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Detached selections retain only the reviewed Store identity. Each operation
// gets a fresh Environment generation under the canonical exclusive Store lease.
func (s *ManagedImages) listDetached(ctx context.Context, id, runtime string) (ManagedImageList, error) {
	if runtime != "nerdctl" || s.Maintain == nil {
		return ManagedImageList{}, core.ErrUnsupported
	}
	resource, err := s.Catalog.GetPersistentResource(ctx, id)
	if err != nil {
		return ManagedImageList{}, err
	}
	target := ImageTarget{Detached: true, Store: resource.Ref(), Runtime: runtime}
	if resource.ID != id {
		return ManagedImageList{}, core.ErrCapabilityStale
	}
	result := ManagedImageList{Target: target}
	err = s.withDetached(ctx, target, func(ctx context.Context, session *ManagedImages, live ImageTarget) error {
		var err error
		result, err = session.inspect(ctx, live)
		result.Target = target
		return err
	})
	return result, err
}

func (s *ManagedImages) withDetached(ctx context.Context, target ImageTarget, operation func(context.Context, *ManagedImages, ImageTarget) error) error {
	if !target.Detached || !validImageTarget(target) {
		return core.ErrInvalidArgument
	}
	if s.Maintain == nil {
		return core.ErrUnsupported
	}
	if err := s.checkTarget(ctx, target); err != nil {
		return err
	}
	return s.Maintain(ctx, target.Store, func(ctx context.Context, env core.Environment) error {
		if env.PersistentResource != target.Store || env.Name == "" || !core.ValidTemporaryWorkspace(env.Workspace) {
			return core.ErrCapabilityStale
		}
		generation, err := s.Catalog.EnvironmentInstance(ctx, env)
		if err != nil {
			return err
		}
		if !core.ValidEnvironmentInstanceID(generation) {
			return core.ErrCapabilityStale
		}
		live := ImageTarget{Environment: env.Name, Instance: generation, Store: target.Store, Runtime: target.Runtime}
		session := *s
		session.metadataOnly = true
		return operation(ctx, &session, live)
	})
}
