package workspace

import (
	"context"
	"reflect"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// withSavedSnapshot is shared by reading and deleting saved data. The lock order
// is the existing Environment then Workspace lifecycle order, including after the
// source Env has gone. It adds no durable recovery state or source-Env dependency.
func (s *Service) withSavedSnapshot(ctx context.Context, id string, use func(context.Context, core.Snapshot) error) error {
	catalog, ok := s.store.(interface {
		GetSnapshot(context.Context, string) (core.Snapshot, error)
	})
	if !ok {
		return core.ErrUnsupported
	}
	observed, err := catalog.GetSnapshot(ctx, id)
	if err != nil {
		return err
	}
	unlock, err := lockLifecycle(ctx, "environment", observed.Source.Environment.Name)
	if err != nil {
		return err
	}
	defer unlock()
	release, err := lockWorkspace(ctx, observed.Source.Environment.Workspace.ID)
	if err != nil {
		return err
	}
	defer release()
	current, err := catalog.GetSnapshot(ctx, id)
	if err != nil {
		return err
	}
	if current.ID != observed.ID || !reflect.DeepEqual(current.Source, observed.Source) {
		return core.ErrCapabilityStale
	}
	return use(ctx, current)
}

// ReadSnapshot holds the canonical deletion locks through verification and use.
// The callback must finish consuming source data before returning; retaining this
// value is not a reservation. Callbacks must not re-enter lifecycle operations.
// Base is historical provenance: its filesystem is not needed to read rootfs.
func (s *Service) ReadSnapshot(ctx context.Context, id string, consume func(context.Context, core.Snapshot) error) error {
	if consume == nil {
		return core.ErrInvalidArgument
	}
	backend, ok := s.runtime.(SnapshotBackend)
	if !ok {
		return core.ErrUnsupported
	}
	return s.withSavedSnapshot(ctx, id, func(ctx context.Context, saved core.Snapshot) error {
		if saved.State != "ready" {
			return core.ErrIncompatibleState
		}
		for _, c := range saved.Components {
			if err := ctx.Err(); err != nil {
				return err
			}
			if c.State != "verified" {
				return core.ErrIncompatibleState
			}
			if c.Role == "base" {
				continue
			}
			if err := backend.VerifySnapshotComponent(ctx, c); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return consume(ctx, saved)
	})
}
