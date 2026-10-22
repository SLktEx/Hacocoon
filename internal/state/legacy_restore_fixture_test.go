package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
)

// Test-only transitions build old-version fixtures; production cannot create staging.
func (s *EnvironmentJSONStore) BeginSnapshotRestore(ctx context.Context, op core.SnapshotRestore) error {
	if validateRestore(op) != nil || op.State != "preparing" {
		return core.ErrInvalidArgument
	}
	for _, c := range op.Components {
		if c.State != "planned" {
			return core.ErrInvalidArgument
		}
	}
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		if _, ok := d.Restores[op.ID]; ok {
			return false, core.ErrAlreadyExists
		}
		if !reflect.DeepEqual(d.Snapshots[op.Saved.ID], op.Saved) || (op.Before.ID != "" && !reflect.DeepEqual(d.Snapshots[op.Before.ID], op.Before)) {
			return false, core.ErrCapabilityStale
		}
		current := op.Current
		lease := d.Leases[current.Environment.Name]
		if !reflect.DeepEqual(d.Environments[current.Environment.Name], current.Environment) || lease.InstanceID != current.InstanceID || lease.State != core.WorkspaceLeaseActive || validateEnvironmentCreateCommit(current.Environment, lease) != nil {
			return false, core.ErrCapabilityStale
		}
		if snapshotBusy(*d, current.Environment.Name) {
			return false, core.ErrStorageBusy
		}
		refs, owners := map[string]bool{}, map[string]bool{}
		for _, snap := range d.Snapshots {
			for _, c := range snap.Components {
				refs[c.NativeRef] = true
				owners[c.Owner] = true
			}
		}
		for _, old := range d.Restores {
			for _, c := range old.Components {
				refs[c.NativeRef] = true
				owners[c.Owner] = true
			}
		}
		for _, a := range d.BaseAssets {
			refs[a.NativeRef] = true
			owners[a.Owner] = true
		}
		for _, env := range d.Environments {
			refs[env.RuntimeRef] = true
		}
		for _, c := range op.Components {
			if refs[c.NativeRef] || owners[c.Owner] {
				return false, core.ErrAlreadyExists
			}
		}
		d.Restores[op.ID] = op
		return true, nil
	})
}
func (s *EnvironmentJSONStore) CommitRestorePreparation(ctx context.Context, id string) error {
	return s.mutateRestore(ctx, id, func(op *core.SnapshotRestore) error {
		if op.State != "preparing" && op.State != "recovery-required" {
			return core.ErrIncompatibleState
		}
		for _, c := range op.Components {
			if c.State != "verified" {
				return core.ErrRecoveryRequired
			}
		}
		op.State = "prepared"
		return nil
	})
}
func (s *EnvironmentJSONStore) MarkRestoreRecovery(ctx context.Context, id string) error {
	return s.mutateRestore(ctx, id, func(op *core.SnapshotRestore) error {
		if op.State != "preparing" && op.State != "recovery-required" {
			return core.ErrIncompatibleState
		}
		op.State = "recovery-required"
		return nil
	})
}
