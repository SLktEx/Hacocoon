package state

import (
	"context"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var snapshotIDPattern = regexp.MustCompile(`^snap-[a-f0-9]{32}$`)
var snapshotOwnerPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func validateSnapshot(s core.Snapshot) error {
	if !snapshotIDPattern.MatchString(s.ID) || !core.ValidEnvironmentInstanceID(s.Source.InstanceID) || s.Source.Environment.Name == "" || s.Source.Environment.RuntimeRef == "" || s.Source.Environment.Workspace.ID == "" || !strings.HasPrefix(s.Source.Environment.Workspace.Path, "managed:") || len(s.Components) < 2 || len(s.Components) > 256 {
		return core.ErrInvalidArgument
	}
	if s.State != "capturing" && s.State != "ready" && s.State != "recovery-required" && s.State != "deleting" {
		return core.ErrInvalidArgument
	}
	refs := map[string]bool{}
	roles := map[string]bool{}
	root, work, oci, base := 0, 0, 0, 0
	for _, c := range s.Components {
		if len(c.Binding) > 16384 || !utf8.ValidString(c.Binding) || c.NativeRef == "" || len(c.NativeRef) > 1024 || len(c.Role) > 1024 || !snapshotOwnerPattern.MatchString(c.Owner) || refs[c.NativeRef] || roles[c.Role] {
			return core.ErrInvalidArgument
		}
		for _, r := range c.NativeRef + c.Role {
			if unicode.IsControl(r) {
				return core.ErrInvalidArgument
			}
		}
		refs[c.NativeRef] = true
		roles[c.Role] = true
		switch {
		case c.Role == "base":
			base++
		case c.Role == "rootfs":
			root++
		case c.Role == "oci":
			oci++
		case strings.HasPrefix(c.Role, "workspace:") && len(c.Role) > 10:
			work++
		default:
			return core.ErrInvalidArgument
		}
		if c.State != "planned" && c.State != "created" && c.State != "verified" && c.State != "absent" {
			return core.ErrInvalidArgument
		}
		if s.State == "ready" && c.State != "verified" {
			return core.ErrInvalidArgument
		}
		if c.State == "absent" && s.State != "deleting" {
			return core.ErrInvalidArgument
		}
	}
	if root != 1 || work == 0 || base > 1 || (base != 0 && (s.Source.Environment.Base == nil || s.Source.Environment.Base.Name == "" || s.Source.Environment.Base.Revision == "")) {
		return core.ErrInvalidArgument
	}
	hasOCI := s.Source.Environment.PersistentResource != (core.PersistentResourceRef{})
	if (hasOCI && (oci != 1 || !core.ValidPersistentResourceRef(s.Source.Environment.PersistentResource))) || (!hasOCI && oci != 0) {
		return core.ErrInvalidArgument
	}
	return nil
}
func snapshotBusy(data environmentFileState, name string) bool {
	for _, op := range data.Restores {
		if op.Current.Environment.Name == name {
			return true
		}
	}
	for _, s := range data.Snapshots {
		if s.Source.Environment.Name == name && s.State != "ready" {
			return true
		}
	}
	return false
}
func (s *EnvironmentJSONStore) CheckSnapshotIdle(ctx context.Context, name string) error {
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		if snapshotBusy(*data, name) {
			return false, core.ErrRecoveryRequired
		}
		return false, nil
	})
}
func (s *EnvironmentJSONStore) BeginSnapshot(ctx context.Context, snapshot core.Snapshot) error {
	if validateSnapshot(snapshot) != nil || snapshot.State != "capturing" {
		return core.ErrInvalidArgument
	}
	for _, c := range snapshot.Components {
		if c.State != "planned" {
			return core.ErrInvalidArgument
		}
	}
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		if _, ok := data.Snapshots[snapshot.ID]; ok {
			return false, core.ErrAlreadyExists
		}
		source := snapshot.Source
		current, ok := data.Environments[source.Environment.Name]
		lease := data.Leases[source.Environment.Name]
		if !ok || !reflect.DeepEqual(current, source.Environment) || lease.InstanceID != source.InstanceID || lease.State != core.WorkspaceLeaseActive || validateEnvironmentCreateCommit(current, lease) != nil {
			return false, core.ErrCapabilityStale
		}
		if snapshotBusy(*data, current.Name) {
			return false, core.ErrStorageBusy
		}
		// Persist intended identities before any provider create; reject shared targets.
		for _, existing := range data.Snapshots {
			for _, a := range existing.Components {
				for _, b := range snapshot.Components {
					if a.NativeRef == b.NativeRef {
						return false, core.ErrAlreadyExists
					}
				}
			}
		}
		data.Snapshots[snapshot.ID] = snapshot
		return true, nil
	})
}
func (s *EnvironmentJSONStore) GetSnapshot(ctx context.Context, id string) (core.Snapshot, error) {
	var result core.Snapshot
	err := s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		value, ok := data.Snapshots[id]
		if !ok {
			return false, core.ErrNotFound
		}
		result = value
		return false, nil
	})
	return result, err
}

// RecordSnapshotComponent takes the exact planned ownership identity. "created"
// must be recorded immediately after provider creation, before verification.
func (s *EnvironmentJSONStore) RecordSnapshotComponent(ctx context.Context, id string, expected core.SnapshotComponent, next string) error {
	return s.mutateSnapshot(ctx, id, func(value *core.Snapshot) error {
		for i, c := range value.Components {
			if c.Role != expected.Role {
				continue
			}
			if c != expected {
				return core.ErrCapabilityStale
			}
			allowed := ((value.State == "capturing" || value.State == "recovery-required") && ((c.State == "planned" && next == "created") || (c.State == "created" && next == "verified"))) ||
				(value.State == "deleting" && next == "absent")
			if !allowed {
				return core.ErrIncompatibleState
			}
			value.Components[i].State = next
			return nil
		}
		return core.ErrNotFound
	})
}
func (s *EnvironmentJSONStore) CommitSnapshot(ctx context.Context, id string) error {
	return s.mutateSnapshot(ctx, id, func(value *core.Snapshot) error {
		if value.State != "capturing" && value.State != "recovery-required" {
			return core.ErrIncompatibleState
		}
		for _, c := range value.Components {
			if c.State != "verified" {
				return core.ErrRecoveryRequired
			}
		}
		value.State = "ready"
		return nil
	})
}
func (s *EnvironmentJSONStore) MarkSnapshotRecovery(ctx context.Context, id string) error {
	return s.mutateSnapshot(ctx, id, func(value *core.Snapshot) error {
		if value.State != "capturing" && value.State != "recovery-required" {
			return core.ErrIncompatibleState
		}
		value.State = "recovery-required"
		return nil
	})
}
func (s *EnvironmentJSONStore) BeginSnapshotDelete(ctx context.Context, id string) error {
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		for _, copy := range d.WorkspaceCopies {
			if copy.SnapshotID == id {
				return false, core.ErrStorageBusy
			}
		}
		for _, r := range d.PersistentResources {
			if r.RestoreSource == id {
				return false, core.ErrStorageBusy
			}
		}
		for _, lease := range d.Leases {
			if lease.SnapshotSource == id {
				return false, core.ErrStorageBusy
			}
		}
		if restoreUsesSnapshot(*d, id) {
			return false, core.ErrStorageBusy
		}
		value, ok := d.Snapshots[id]
		if !ok {
			return false, core.ErrNotFound
		}
		value.State = "deleting"
		d.Snapshots[id] = value
		return true, nil
	})
}
func (s *EnvironmentJSONStore) FinalizeSnapshotDelete(ctx context.Context, id string) error {
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		value, ok := data.Snapshots[id]
		if !ok {
			return false, core.ErrNotFound
		}
		if value.State != "deleting" {
			return false, core.ErrIncompatibleState
		}
		for _, c := range value.Components {
			if c.State != "absent" {
				return false, core.ErrRecoveryRequired
			}
		}
		delete(data.Snapshots, id)
		return true, nil
	})
}
func (s *EnvironmentJSONStore) mutateSnapshot(ctx context.Context, id string, change func(*core.Snapshot) error) error {
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		value, ok := data.Snapshots[id]
		if !ok {
			return false, core.ErrNotFound
		}
		if err := change(&value); err != nil {
			return false, err
		}
		if err := validateSnapshot(value); err != nil {
			return false, err
		}
		data.Snapshots[id] = value
		return true, nil
	})
}

func (s *EnvironmentJSONStore) ListSnapshots(ctx context.Context) ([]core.Snapshot, error) {
	result := []core.Snapshot{}
	err := s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		for _, saved := range d.Snapshots {
			result = append(result, saved)
		}
		return false, nil
	})
	sort.Slice(result, func(i, j int) bool {
		if result[i].Source.Environment.Name != result[j].Source.Environment.Name {
			return result[i].Source.Environment.Name < result[j].Source.Environment.Name
		}
		return result[i].ID < result[j].ID
	})
	return result, err
}
