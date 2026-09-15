package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (b *PersistentResourceBackend) savedOCI(saved core.Snapshot) (*snapshotVolumePlan, error) {
	if saved.State != "ready" {
		return nil, core.ErrIncompatibleState
	}
	var result *snapshotVolumePlan
	for _, c := range saved.Components {
		if c.Role != "oci" {
			continue
		}
		binding, err := b.Runtime.decodeSavedComponent(c)
		if err != nil {
			return nil, err
		}
		if result != nil || binding.Volume == nil || binding.Volume.SourceKind != OCIStoreKind || c.State != "verified" {
			return nil, core.ErrIncompatibleState
		}
		result = binding.Volume
	}
	if result == nil {
		return nil, core.ErrNotFound
	}
	return result, nil
}
func (b *PersistentResourceBackend) PlanSavedResource(ctx context.Context, saved core.Snapshot, owner string) (string, string, error) {
	p, err := b.savedOCI(saved)
	if err != nil {
		return "", "", err
	}
	if !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:restore", Owner: owner}) || owner == p.Owner || owner == p.SourceOwner {
		return "", "", core.ErrInvalidArgument
	}
	if err := b.Runtime.verifySnapshotVolume(ctx, *p); err != nil {
		return "", "", err
	}
	return OCIStoreKind, p.Pool + "/haco-persistent-" + owner, nil
}
func (b *PersistentResourceBackend) CreateSavedResource(ctx context.Context, saved core.Snapshot, target core.PersistentResource) error {
	p, err := b.savedOCI(saved)
	if err != nil {
		return err
	}
	pool, name, err := persistentVolume(target)
	if err != nil {
		return err
	}
	if target.SourceOnly || target.State != "creating" || target.RestoreSource != saved.ID || target.CopySource != (core.PersistentResourceRef{}) || target.CopyCompleted || target.Owner == p.Owner || target.Owner == p.SourceOwner || pool != p.Pool {
		return core.ErrCapabilityStale
	}
	config := map[string]string{"user.hacocoon.owner": target.Owner, "user.hacocoon.resource": target.ID, "user.hacocoon.kind": OCIStoreKind, "user.hacocoon.source-only": "false"}
	return b.Runtime.copySavedVolume(ctx, p, pool, name, config)
}
