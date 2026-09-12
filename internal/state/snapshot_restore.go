package state

import (
	"context"
	"reflect"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var restoreIDPattern = regexp.MustCompile(`^restore-[a-f0-9]{32}$`)

func validateRestore(op core.SnapshotRestore) error {
	if !restoreIDPattern.MatchString(op.ID) || validateSnapshot(op.Saved) != nil || op.Saved.State != "ready" {
		return core.ErrInvalidArgument
	}
	if !core.ValidEnvironmentInstanceID(op.Current.InstanceID) || op.Current.Environment.Name == "" || op.Current.Environment.Workspace.ID == "" {
		return core.ErrInvalidArgument
	}
	if op.Before.ID == "" && !reflect.DeepEqual(op.Before, core.Snapshot{}) {
		return core.ErrInvalidArgument
	}
	if op.Before.ID != "" && (validateSnapshot(op.Before) != nil || op.Before.State != "ready" || op.Before.ID == op.Saved.ID || !reflect.DeepEqual(op.Before.Source, op.Current)) {
		return core.ErrInvalidArgument
	}
	if op.State != "preparing" && op.State != "prepared" && op.State != "recovery-required" && op.State != "deleting" {
		return core.ErrInvalidArgument
	}
	shape := core.Snapshot{ID: "snap-" + strings.TrimPrefix(op.ID, "restore-"), Source: op.Saved.Source, Components: op.Components, State: "capturing"}
	if op.State == "prepared" {
		shape.State = "ready"
	}
	if op.State == "deleting" {
		shape.State = "deleting"
	}
	if validateSnapshot(shape) != nil {
		return core.ErrInvalidArgument
	}
	roles, refs, owners := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, snap := range []core.Snapshot{op.Saved, op.Before} {
		for _, c := range snap.Components {
			if c.Binding == "" {
				return core.ErrUnsupported
			}
			refs[c.NativeRef] = true
			owners[c.Owner] = true
		}
	}
	for _, c := range op.Saved.Components {
		if c.Role != "base" || op.Before.ID != "" {
			roles[c.Role] = true
		}
	}
	if len(op.Components) != len(roles) {
		return core.ErrInvalidArgument
	}
	for _, c := range op.Components {
		if !roles[c.Role] || c.Binding == "" || refs[c.NativeRef] || owners[c.Owner] {
			return core.ErrInvalidArgument
		}
		owners[c.Owner] = true
	}
	return nil
}

func restoreUsesSnapshot(data environmentFileState, id string) bool {
	for _, op := range data.Restores {
		if op.Saved.ID == id || op.Before.ID == id {
			return true
		}
	}
	return false
}

// BeginSnapshotRestore reserves the saved data and current target identity.
// No current data is copied or replaced by this transition.
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
func (s *EnvironmentJSONStore) GetSnapshotRestore(ctx context.Context, id string) (core.SnapshotRestore, error) {
	var op core.SnapshotRestore
	err := s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		var ok bool
		op, ok = d.Restores[id]
		if !ok {
			return false, core.ErrNotFound
		}
		return false, nil
	})
	return op, err
}
func (s *EnvironmentJSONStore) mutateRestore(ctx context.Context, id string, change func(*core.SnapshotRestore) error) error {
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		op, ok := d.Restores[id]
		if !ok {
			return false, core.ErrNotFound
		}
		if err := change(&op); err != nil {
			return false, err
		}
		if err := validateRestore(op); err != nil {
			return false, err
		}
		d.Restores[id] = op
		return true, nil
	})
}
func (s *EnvironmentJSONStore) RecordRestoreComponent(ctx context.Context, id string, expected core.SnapshotComponent, next string) error {
	return s.mutateRestore(ctx, id, func(op *core.SnapshotRestore) error {
		for i, c := range op.Components {
			if c.Role != expected.Role {
				continue
			}
			if c != expected {
				return core.ErrCapabilityStale
			}
			allowed := ((op.State == "preparing" || op.State == "recovery-required") && ((c.State == "planned" && next == "created") || (c.State == "created" && next == "verified"))) || (op.State == "deleting" && next == "absent")
			if !allowed {
				return core.ErrIncompatibleState
			}
			op.Components[i].State = next
			return nil
		}
		return core.ErrNotFound
	})
}

// CommitRestorePreparation publishes only complete staging, never an Environment.
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
func (s *EnvironmentJSONStore) BeginRestoreCleanup(ctx context.Context, id string) error {
	return s.mutateRestore(ctx, id, func(op *core.SnapshotRestore) error { op.State = "deleting"; return nil })
}
func (s *EnvironmentJSONStore) FinalizeRestoreCleanup(ctx context.Context, id string) error {
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		op, ok := d.Restores[id]
		if !ok {
			return false, core.ErrNotFound
		}
		if op.State != "deleting" {
			return false, core.ErrIncompatibleState
		}
		for _, c := range op.Components {
			if c.State != "absent" {
				return false, core.ErrRecoveryRequired
			}
		}
		delete(d.Restores, id)
		return true, nil
	})
}
