package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *EnvironmentJSONStore) BeginEnvironmentCreateFromSnapshotWithResources(ctx context.Context, lease core.WorkspaceLease, saved core.Snapshot, plans []core.EnvironmentResourcePlan) error {
	if len(plans) == 0 || validateSnapshot(saved) != nil || saved.State != "ready" || lease.SnapshotSource != saved.ID || !core.ValidEnvironmentInstanceID(lease.InstanceID) || lease.InstanceID == saved.Source.InstanceID || len(plans) != len(saved.Source.Environment.Attachments) {
		return core.ErrInvalidArgument
	}
	for _, plan := range plans {
		if plan.Resource.RestoreSource != saved.ID {
			return core.ErrInvalidArgument
		}
	}
	return s.beginEnvironmentCreate(ctx, lease, &saved, plans)
}
func matchesSavedEnvironmentResource(data environmentFileState, lease core.WorkspaceLease, a core.EnvironmentAttachment, r core.PersistentResource) bool {
	saved, ok := data.Snapshots[r.RestoreSource]
	if !ok || saved.State != "ready" || r.RestoreSource != lease.SnapshotSource || r.CopySource != (core.PersistentResourceRef{}) || r.CopyCompleted {
		return false
	}
	for _, source := range saved.Source.Environment.Attachments {
		if source.Key == a.Key && source.Target == a.Target && source.Origin == a.Origin && source.Resource != a.Resource && source.Resource.Owner != r.Owner {
			return true
		}
	}
	return false
}

func validSavedEnvironmentLease(data environmentFileState, lease core.WorkspaceLease) bool {
	saved, ok := data.Snapshots[lease.SnapshotSource]
	if !ok || saved.State != "ready" || lease.InstanceID == saved.Source.InstanceID || len(lease.Attachments) != len(saved.Source.Environment.Attachments) || (lease.State != core.WorkspaceLeaseAcquiring && lease.State != core.WorkspaceLeaseCleanupRequired) {
		return false
	}
	for i, a := range lease.Attachments {
		original := saved.Source.Environment.Attachments[i]
		if original.Key != a.Key || original.Target != a.Target || original.Origin != a.Origin || original.Resource == a.Resource || original.Resource.Owner == a.Resource.Owner {
			return false
		}
	}
	return true
}
