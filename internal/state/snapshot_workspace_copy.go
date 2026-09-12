package state

import (
	"context"
	"reflect"
	"regexp"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// The registry holds exact destination volume receipts. This catalog reference
// prevents saved-data deletion until publication or positive owned cleanup.
type snapshotWorkspaceCopy struct {
	SnapshotID string `json:"snapshot_id"`
	Owner      string `json:"owner"`
}

var snapshotWorkspaceID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

func validSnapshotWorkspaceCopy(id string, copy snapshotWorkspaceCopy) bool {
	return snapshotWorkspaceID.MatchString(id) && snapshotIDPattern.MatchString(copy.SnapshotID) && core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: copy.Owner})
}

func (s *EnvironmentJSONStore) BeginSnapshotWorkspaceCopy(ctx context.Context, saved core.Snapshot, id, owner string) error {
	copy := snapshotWorkspaceCopy{SnapshotID: saved.ID, Owner: owner}
	if !validSnapshotWorkspaceCopy(id, copy) || saved.State != "ready" {
		return core.ErrInvalidArgument
	}
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		if current, ok := d.Snapshots[saved.ID]; !ok || !reflect.DeepEqual(current, saved) {
			return false, core.ErrCapabilityStale
		}
		if _, exists := d.WorkspaceCopies[id]; exists {
			return false, core.ErrAlreadyExists
		}
		d.WorkspaceCopies[id] = copy
		return true, nil
	})
}

// FinishSnapshotWorkspaceCopy is called only after durable publication or after
// every exact-owned destination is positively absent. A stale owner cannot
// release a newer copy's source reservation.
func (s *EnvironmentJSONStore) FinishSnapshotWorkspaceCopy(ctx context.Context, savedID, id, owner string) error {
	expected := snapshotWorkspaceCopy{SnapshotID: savedID, Owner: owner}
	if !validSnapshotWorkspaceCopy(id, expected) {
		return core.ErrInvalidArgument
	}
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		current, exists := d.WorkspaceCopies[id]
		if !exists {
			return false, core.ErrNotFound
		}
		if current != expected {
			return false, core.ErrCapabilityStale
		}
		delete(d.WorkspaceCopies, id)
		return true, nil
	})
}
