package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (b *PersistentResourceBackend) savedEnvironmentData(saved core.Snapshot, key string) (*snapshotVolumePlan, error) {
	if saved.State != "ready" {
		return nil, core.ErrIncompatibleState
	}
	var source *core.EnvironmentAttachment
	for _, a := range saved.Source.Environment.Attachments {
		if a.Key == key {
			copy := a
			source = &copy
		}
	}
	if source == nil {
		return nil, core.ErrNotFound
	}
	var result *snapshotVolumePlan
	for _, c := range saved.Components {
		if c.Role != "data:"+key {
			continue
		}
		binding, err := b.Runtime.decodeSavedComponent(c)
		if err != nil {
			return nil, err
		}
		p := binding.Volume
		if result != nil || p == nil || c.State != "verified" || p.SourceKind != CacheResourceKind || p.SourceKind != source.Origin.Kind || p.SourceID != source.Resource.ID || p.SourceOwner != source.Resource.Owner || p.Path != source.Target || p.SourceInstanceID != saved.Source.InstanceID {
			return nil, core.ErrCapabilityStale
		}
		result = p
	}
	if result == nil {
		return nil, core.ErrNotFound
	}
	return result, nil
}
func (b *PersistentResourceBackend) PlanSavedEnvironmentResource(ctx context.Context, saved core.Snapshot, key, owner string) (string, string, error) {
	p, err := b.savedEnvironmentData(saved, key)
	if err != nil {
		return "", "", err
	}
	if !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "env-data:00000000000000000000000000000000", Owner: owner}) || owner == p.Owner || owner == p.SourceOwner {
		return "", "", core.ErrInvalidArgument
	}
	if err := b.Runtime.verifySnapshotVolume(ctx, *p); err != nil {
		return "", "", err
	}
	return CacheResourceKind, p.Pool + "/haco-persistent-" + owner, nil
}
func (b *PersistentResourceBackend) CreateSavedEnvironmentResource(ctx context.Context, saved core.Snapshot, key string, target core.PersistentResource) error {
	p, err := b.savedEnvironmentData(saved, key)
	if err != nil {
		return err
	}
	pool, name, err := managedResourceVolume(target)
	if err != nil {
		return err
	}
	if target.Kind != CacheResourceKind || !core.ValidEnvironmentInstanceID(target.EnvironmentInstance) || target.EnvironmentInstance == saved.Source.InstanceID || target.SourceOnly || target.State != "creating" || target.RestoreSource != saved.ID || target.CopySource != (core.PersistentResourceRef{}) || target.CopyCompleted || target.Owner == p.Owner || target.Owner == p.SourceOwner || pool != p.Pool {
		return core.ErrCapabilityStale
	}
	return b.Runtime.copySavedVolume(ctx, p, pool, name, persistentResourceConfig(target))
}
