package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type savedEnvironmentResourceCatalog interface {
	BeginEnvironmentCreateFromSnapshotWithResources(context.Context, core.WorkspaceLease, core.Snapshot, []core.EnvironmentResourcePlan) error
}

func (s *Service) planSavedEnvironmentResources(ctx context.Context, spec core.EnvironmentSpec, work core.Workspace, lease core.WorkspaceLease, saved core.Snapshot) ([]core.EnvironmentResourcePlan, error) {
	if len(saved.Source.Environment.Attachments) == 0 {
		return nil, nil
	}
	manager, ok := s.environmentResources.(interface {
		PlanSavedEnvironmentResources(context.Context, core.EnvironmentResourceRequest, core.Snapshot) ([]core.EnvironmentResourcePlan, error)
	})
	if !ok {
		return nil, core.ErrUnsupported
	}
	if _, ok := s.store.(savedEnvironmentResourceCatalog); !ok {
		return nil, core.ErrUnsupported
	}
	return manager.PlanSavedEnvironmentResources(ctx, core.EnvironmentResourceRequest{EnvironmentID: lease.EnvironmentID, InstanceID: lease.InstanceID, Workspace: work, Base: spec.Base, ReadOnly: lease.AccessMode == core.WorkspaceReadOnly}, saved)
}
