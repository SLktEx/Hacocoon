package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
)

func (s *EnvironmentJSONStore) BeginSnapshotResourceRestore(_ context.Context, saved core.Snapshot, r core.PersistentResource) error {
	if !core.ValidPersistentResourceRef(r.Ref()) || r.Kind == "" || r.NativeRef == "" || r.State != "creating" || r.CreatedAt.IsZero() || r.SourceOnly || r.CopyCompleted || r.CopySource != (core.PersistentResourceRef{}) || r.RestoreSource != saved.ID || saved.State != "ready" {
		return core.ErrInvalidArgument
	}
	return s.resourceTransaction(func(d *environmentFileState) error {
		if current, ok := d.Snapshots[saved.ID]; !ok || !reflect.DeepEqual(current, saved) {
			return core.ErrCapabilityStale
		}
		if _, exists := d.PersistentResources[r.ID]; exists {
			return core.ErrAlreadyExists
		}
		d.PersistentResources[r.ID] = r
		return nil
	})
}
func (s *EnvironmentJSONStore) RecordPersistentResourceCreated(_ context.Context, r core.PersistentResource) error {
	if r.State != "creating" || r.RestoreSource == "" {
		return core.ErrInvalidArgument
	}
	return s.resourceTransaction(func(d *environmentFileState) error {
		if current, ok := d.PersistentResources[r.ID]; !ok || current != r {
			return core.ErrCapabilityStale
		}
		r.State = "created"
		d.PersistentResources[r.ID] = r
		return nil
	})
}
func (s *EnvironmentJSONStore) BeginPersistentResourceDeleteOwned(ctx context.Context, r core.PersistentResource) (core.PersistentResource, error) {
	if !core.ValidPersistentResourceRef(r.Ref()) {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	return s.beginPersistentResourceDelete(ctx, r.ID, "", r.Owner)
}
