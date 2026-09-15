package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type environmentResourceImporter interface {
	PlanImportedEnvironmentResources(context.Context, core.EnvironmentResourceRequest, []core.EnvironmentResourceImport) ([]core.EnvironmentResourcePlan, error)
	ImportEnvironmentResources(context.Context, core.WorkspaceLease, []core.EnvironmentResourceImport) ([]core.EnvironmentRuntimeAttachment, error)
}

func (s *Service) planImportedEnvironmentResources(ctx context.Context, work core.Workspace, lease core.WorkspaceLease, inputs []core.EnvironmentResourceImport) ([]core.EnvironmentResourcePlan, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	manager, ok := s.environmentResources.(environmentResourceImporter)
	if !ok {
		return nil, core.ErrUnsupported
	}
	provider, ok := s.runtime.(interface{ SupportsEnvironmentResources() bool })
	if !ok || !provider.SupportsEnvironmentResources() {
		return nil, core.ErrUnsupported
	}
	if _, ok := s.store.(environmentResourceCatalog); !ok {
		return nil, core.ErrUnsupported
	}
	return manager.PlanImportedEnvironmentResources(ctx, core.EnvironmentResourceRequest{EnvironmentID: lease.EnvironmentID, InstanceID: lease.InstanceID, Workspace: work, ReadOnly: lease.AccessMode == core.WorkspaceReadOnly}, inputs)
}
