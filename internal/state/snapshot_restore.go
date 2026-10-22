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

// Legacy staging records are retained only for fail-closed ownership cleanup.
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
