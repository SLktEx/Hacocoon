package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// EnvironmentResources owns provider data mechanics, while Service retains the
// single Env/Workspace lifecycle and its positive-absence cleanup decision.
type EnvironmentResources interface {
	PlanEnvironmentResources(context.Context, core.EnvironmentResourceRequest, []core.EnvironmentResourceSelection) ([]core.EnvironmentResourcePlan, error)
	MaterializeEnvironmentResources(context.Context, core.WorkspaceLease) ([]core.EnvironmentRuntimeAttachment, error)
	DeleteEnvironmentResources(context.Context, core.WorkspaceLease) error
}

type environmentResourceCatalog interface {
	BeginEnvironmentCreateWithResources(context.Context, core.WorkspaceLease, []core.EnvironmentResourcePlan) error
	PrepareEnvironmentResourceDeletion(context.Context, core.WorkspaceLease) (core.WorkspaceLease, error)
}

// Configure once before requests. Selection is trusted Host policy, never guest
// or client-supplied provider identities or arbitrary mount instructions.
func (s *Service) ConfigureEnvironmentResources(manager EnvironmentResources, selectAreas func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error)) {
	s.environmentResources = manager
	s.selectEnvironmentResources = selectAreas
}

func (s *Service) planEnvironmentResources(ctx context.Context, spec core.EnvironmentSpec, work core.Workspace, lease core.WorkspaceLease, saved bool) ([]core.EnvironmentResourcePlan, error) {
	// Tool maintenance does not enroll scratch Workspaces into ordinary-Env caches.
	if s.selectEnvironmentResources == nil || spec.TemporaryWorkspace != nil {
		return nil, nil
	}
	request := core.EnvironmentResourceRequest{EnvironmentID: lease.EnvironmentID, InstanceID: lease.InstanceID, Workspace: work, Base: spec.Base, ReadOnly: lease.AccessMode == core.WorkspaceReadOnly}
	selected, err := s.selectEnvironmentResources(ctx, request)
	if err != nil || len(selected) == 0 {
		return nil, err
	}
	if saved {
		return nil, core.ErrUnsupported
	}
	provider, ok := s.runtime.(interface{ SupportsEnvironmentResources() bool })
	if !ok || !provider.SupportsEnvironmentResources() || s.environmentResources == nil {
		return nil, core.ErrUnsupported
	}
	if _, ok := s.store.(environmentResourceCatalog); !ok {
		return nil, core.ErrUnsupported
	}
	return s.environmentResources.PlanEnvironmentResources(ctx, request, selected)
}

// Called only before any runtime request or after complete runtime deletion.
// The durable absence fence prevents new child creation, and failed child
// cleanup keeps both the Env identity and Workspace reservation available.
func (s *Service) finalizeAbsentEnvironment(ctx context.Context, name string) error {
	lease, err := s.store.GetWorkspaceLease(ctx, name)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return err
	}
	if len(lease.Attachments) != 0 {
		catalog, ok := s.store.(environmentResourceCatalog)
		if !ok || s.environmentResources == nil {
			return core.ErrRecoveryRequired
		}
		absent, err := catalog.PrepareEnvironmentResourceDeletion(ctx, lease)
		if err != nil {
			return errors.Join(err, core.ErrRecoveryRequired)
		}
		if err := s.environmentResources.DeleteEnvironmentResources(ctx, absent); err != nil {
			return errors.Join(err, core.ErrRecoveryRequired)
		}
	}
	if err := s.store.FinalizeEnvironmentDelete(ctx, name); err != nil {
		return fmt.Errorf("finalize environment data: %w: %w", core.ErrRecoveryRequired, err)
	}
	return nil
}
