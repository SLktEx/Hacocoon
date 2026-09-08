package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type snapshotCreationCatalog interface {
	GetSnapshot(context.Context, string) (core.Snapshot, error)
	BeginEnvironmentCreateFromSnapshot(context.Context, core.WorkspaceLease, core.Snapshot) error
}
type snapshotRuntimeCreator interface {
	CreateEnvironmentFromSnapshot(context.Context, core.EnvironmentRuntimeSpec, core.Snapshot, func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)
}

// CreateFromSnapshot uses saved rootfs with caller-prepared normal data bindings.
// Aggregate data-copy orchestration is separate. Canonical creation owns the
// source reservation, generation, runtime receipt, publication and failed cleanup.
func (s *Service) CreateFromSnapshot(ctx context.Context, spec core.EnvironmentSpec, savedID string) (core.Environment, error) {
	catalog, ok := s.store.(snapshotCreationCatalog)
	if !ok {
		return core.Environment{}, core.ErrUnsupported
	}
	if _, ok := s.runtime.(snapshotRuntimeCreator); !ok {
		return core.Environment{}, core.ErrUnsupported
	}
	if spec.Base != "" || spec.TemporaryWorkspace != nil {
		return core.Environment{}, core.ErrInvalidArgument
	}
	saved, err := catalog.GetSnapshot(ctx, savedID)
	if err != nil {
		return core.Environment{}, err
	}
	if saved.State != "ready" {
		return core.Environment{}, core.ErrIncompatibleState
	}
	if (saved.Source.Environment.PersistentResource.ID != "") != (spec.PersistentResource != "") {
		return core.Environment{}, core.ErrInvalidArgument
	}
	// Restoring an aggregate never silently copies the current Host OCI area.
	spec.SkipDefaultResource = spec.PersistentResource == ""
	return s.create(ctx, spec, &saved)
}
