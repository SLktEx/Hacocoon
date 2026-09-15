package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// BeginEnvironmentResourceClear fences all writers before clearing contents.
// It never releases the resource, parent identity or Workspace/OCI ownership.
func (s *EnvironmentJSONStore) BeginEnvironmentResourceClear(ctx context.Context, expected core.WorkspaceLease, ref core.PersistentResourceRef) (result core.PersistentResource, err error) {
	if !core.ValidEnvironmentResourceRef(ref) {
		return result, core.ErrInvalidArgument
	}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		lease, ok := data.Leases[expected.EnvironmentID]
		if !ok || !lease.Equal(expected) || lease.State != core.WorkspaceLeaseActive || lease.RuntimeAbsent || lease.AccessMode != core.WorkspaceReadWrite || !lease.MatchesEnvironment(data.Environments[lease.EnvironmentID]) {
			return false, core.ErrCapabilityStale
		}
		r, ok := data.PersistentResources[ref.ID]
		parent, bound := environmentResourceLease(*data, r)
		if !ok || r.Ref() != ref || !bound || !parent.Equal(lease) {
			return false, core.ErrCapabilityStale
		}
		if snapshotBusy(*data, lease.EnvironmentID) {
			return false, core.ErrRecoveryRequired
		}
		for _, a := range lease.Attachments {
			if persistentCopyReserved(*data, a.Resource.ID) || a.Resource != ref && data.PersistentResources[a.Resource.ID].State == "clearing" {
				return false, core.ErrRecoveryRequired
			}
		}
		if r.State != "ready" && r.State != "clearing" {
			return false, core.ErrRecoveryRequired
		}
		if r.State == "clearing" {
			result = r
			return false, nil
		}
		r.State = "clearing"
		data.PersistentResources[r.ID] = r
		result = r
		return true, nil
	})
	return
}

// Commit only after the provider positively verified empty contents and exact
// ownership. An unknown/partial clear stays fenced for explicit fresh review.
func (s *EnvironmentJSONStore) CommitEnvironmentResourceClear(ctx context.Context, expected core.WorkspaceLease, resource core.PersistentResource) error {
	if resource.State != "clearing" || !core.ValidEnvironmentResourceRef(resource.Ref()) {
		return core.ErrInvalidArgument
	}
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		lease, ok := data.Leases[expected.EnvironmentID]
		if !ok || !lease.Equal(expected) || lease.State != core.WorkspaceLeaseActive || lease.RuntimeAbsent || data.PersistentResources[resource.ID] != resource {
			return false, core.ErrCapabilityStale
		}
		parent, bound := environmentResourceLease(*data, resource)
		if !bound || !parent.Equal(lease) {
			return false, core.ErrCapabilityStale
		}
		resource.State = "ready"
		data.PersistentResources[resource.ID] = resource
		return true, nil
	})
}
