package persistentresource

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type savedEnvironmentBackend interface {
	PlanSavedEnvironmentResource(context.Context, core.Snapshot, string, string) (string, string, error)
	CreateSavedEnvironmentResource(context.Context, core.Snapshot, string, core.PersistentResource) error
}

func (s *Service) PlanSavedEnvironmentResources(ctx context.Context, request core.EnvironmentResourceRequest, saved core.Snapshot) ([]core.EnvironmentResourcePlan, error) {
	backend, ok := s.Backend.(savedEnvironmentBackend)
	if !ok {
		return nil, core.ErrUnsupported
	}
	if saved.State != "ready" || !core.ValidEnvironmentAttachments(saved.Source.Environment.Attachments) {
		return nil, core.ErrInvalidArgument
	}
	selections := make([]core.EnvironmentResourceSelection, 0, len(saved.Source.Environment.Attachments))
	for _, a := range saved.Source.Environment.Attachments {
		selections = append(selections, core.EnvironmentResourceSelection{Key: a.Key, Target: a.Target, Origin: a.Origin})
	}
	plans, err := s.PlanEnvironmentResources(ctx, request, selections)
	if err != nil {
		return nil, err
	}
	for i := range plans {
		plan := &plans[i]
		kind, native, err := backend.PlanSavedEnvironmentResource(ctx, saved, plan.Attachment.Key, plan.Resource.Owner)
		if err != nil {
			return nil, err
		}
		if kind != plan.Resource.Kind || native == "" {
			return nil, core.ErrIncompatibleState
		}
		plan.Resource.NativeRef = native
		plan.Resource.CopySource = core.PersistentResourceRef{}
		plan.Resource.RestoreSource = saved.ID
	}
	return plans, nil
}
func (s *Service) materializeSavedEnvironmentResource(ctx context.Context, lease core.WorkspaceLease, a core.EnvironmentAttachment, r core.PersistentResource) (core.PersistentResource, error) {
	catalog, ok := s.Store.(interface {
		GetSnapshot(context.Context, string) (core.Snapshot, error)
	})
	if !ok {
		return r, core.ErrUnsupported
	}
	backend, ok := s.Backend.(savedEnvironmentBackend)
	if !ok {
		return r, core.ErrUnsupported
	}
	if r.RestoreSource != lease.SnapshotSource || r.CopySource != (core.PersistentResourceRef{}) {
		return r, core.ErrCapabilityStale
	}
	saved, err := catalog.GetSnapshot(ctx, r.RestoreSource)
	if err != nil {
		return r, err
	}
	// Override only the provider create mechanic; receipt, verification, commit and
	// positive-absence cleanup remain the canonical shared implementation.
	restoring := Service{Store: s.Store, Backend: importBackend{Backend: s.Backend, create: func(ctx context.Context, target core.PersistentResource) error {
		return backend.CreateSavedEnvironmentResource(ctx, saved, a.Key, target)
	}}}
	return restoring.createReserved(ctx, r, nil)
}
