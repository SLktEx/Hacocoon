package state

import (
	"context"
	"slices"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// BeginEnvironmentCreateWithResources reserves planned disposable data and copy
// sources in the same transaction as the Environment identity and Workspace.
func (s *EnvironmentJSONStore) BeginEnvironmentCreateWithResources(ctx context.Context, lease core.WorkspaceLease, plans []core.EnvironmentResourcePlan) error {
	if lease.SnapshotSource != "" || len(plans) == 0 {
		return core.ErrInvalidArgument
	}
	return s.beginEnvironmentCreate(ctx, lease, nil, plans)
}

func reserveEnvironmentResources(data *environmentFileState, lease core.WorkspaceLease, plans []core.EnvironmentResourcePlan) error {
	if len(plans) != len(lease.Attachments) {
		return core.ErrInvalidArgument
	}
	for i, plan := range plans {
		a, r := plan.Attachment, plan.Resource
		if a != lease.Attachments[i] || !core.ValidEnvironmentResourceRef(r.Ref()) || r.Ref() != a.Resource || r.EnvironmentInstance != lease.InstanceID || r.State != "planned" || r.SourceOnly || r.WorkspaceID != "" || r.RestoreSource != "" || r.CopyCompleted || r.Kind != a.Origin.Kind || r.NativeRef == "" || r.CreatedAt.IsZero() {
			return core.ErrInvalidArgument
		}
		if current, ok := data.ResourceGenerations[a.Origin.Name]; !ok || current != a.Origin {
			return core.ErrSourceGenerationStale
		}
		if r.CopySource != a.Origin.Current {
			return core.ErrInvalidArgument
		}
		if _, ok := data.PersistentResources[r.ID]; ok {
			return core.ErrAlreadyExists
		}
		for _, other := range data.PersistentResources {
			if other.NativeRef == r.NativeRef || other.Owner == r.Owner {
				return core.ErrIncompatibleState
			}
		}
		if r.CopySource != (core.PersistentResourceRef{}) {
			source, ok := data.PersistentResources[r.CopySource.ID]
			if !ok || source.Ref() != r.CopySource || !source.SourceOnly {
				return core.ErrIncompatibleState
			}
			if err := reservePersistentResourceCopy(data, source, r); err != nil {
				return err
			}
		} else {
			data.PersistentResources[r.ID] = r
		}
	}
	return nil
}

func environmentResourceLease(data environmentFileState, resource core.PersistentResource) (core.WorkspaceLease, bool) {
	if resource.EnvironmentInstance == "" {
		return core.WorkspaceLease{}, false
	}
	for _, lease := range data.Leases {
		if lease.InstanceID != resource.EnvironmentInstance {
			continue
		}
		for _, a := range lease.Attachments {
			if a.Resource == resource.Ref() {
				return lease, true
			}
		}
	}
	return core.WorkspaceLease{}, false
}

// Claim the planned creation exactly once before issuing a provider request.
func (s *EnvironmentJSONStore) BeginEnvironmentResourceMaterialization(ctx context.Context, expected core.WorkspaceLease, ref core.PersistentResourceRef) (result core.PersistentResource, err error) {
	if !core.ValidEnvironmentResourceRef(ref) {
		return result, core.ErrInvalidArgument
	}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		lease, ok := data.Leases[expected.EnvironmentID]
		if !ok || !lease.Equal(expected) || lease.State != core.WorkspaceLeaseAcquiring || lease.RuntimeAbsent || lease.RuntimeRef != "" {
			return false, core.ErrCapabilityStale
		}
		r, ok := data.PersistentResources[ref.ID]
		parent, bound := environmentResourceLease(*data, r)
		if !ok || r.Ref() != ref || !bound || !parent.Equal(lease) {
			return false, core.ErrIncompatibleState
		}
		if r.State != "planned" {
			return false, core.ErrRecoveryRequired
		}
		r.State = "creating"
		data.PersistentResources[r.ID] = r
		result = r
		return true, nil
	})
	return
}

// A positive create receipt permits cleanup without confusing an unfinished
// backend request with an empty, completed volume.
func (s *EnvironmentJSONStore) RecordEnvironmentResourceCreated(ctx context.Context, expected core.PersistentResource) (result core.PersistentResource, err error) {
	if expected.EnvironmentInstance == "" || expected.State != "creating" || expected.CopySource != (core.PersistentResourceRef{}) {
		return result, core.ErrInvalidArgument
	}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		r, ok := data.PersistentResources[expected.ID]
		lease, bound := environmentResourceLease(*data, r)
		if !ok || r != expected || !bound || lease.State != core.WorkspaceLeaseAcquiring || lease.RuntimeAbsent {
			return false, core.ErrCapabilityStale
		}
		r.State = "created"
		data.PersistentResources[r.ID] = r
		result = r
		return true, nil
	})
	return
}

// The lifecycle owner calls this only after complete runtime absence, or before
// any runtime request was issued. Reservations remain held through data cleanup.
func (s *EnvironmentJSONStore) PrepareEnvironmentResourceDeletion(ctx context.Context, expected core.WorkspaceLease) (result core.WorkspaceLease, err error) {
	if len(expected.Attachments) == 0 || !core.ValidEnvironmentInstanceID(expected.InstanceID) {
		return result, core.ErrInvalidArgument
	}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		lease, ok := data.Leases[expected.EnvironmentID]
		if !ok || !lease.Equal(expected) {
			return false, core.ErrCapabilityStale
		}
		if snapshotBusy(*data, lease.EnvironmentID) {
			return false, core.ErrRecoveryRequired
		}
		if lease.RuntimeAbsent {
			result = lease
			return false, nil
		}
		lease.RuntimeAbsent = true
		lease.State = core.WorkspaceLeaseCleanupRequired
		data.Leases[lease.EnvironmentID] = lease
		result = lease
		return true, nil
	})
	return
}

func (s *EnvironmentJSONStore) BeginEnvironmentResourceDelete(ctx context.Context, instance string, ref core.PersistentResourceRef) (core.PersistentResource, error) {
	if !core.ValidEnvironmentInstanceID(instance) || !core.ValidEnvironmentResourceRef(ref) {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	return s.beginPersistentResourceDelete(ctx, ref.ID, persistentResourceDeletion{Identity: ref, EnvironmentInstance: instance})
}

func leaseHasResource(lease core.WorkspaceLease, id string) bool {
	if lease.PersistentResource.ID == id {
		return true
	}
	for _, a := range lease.Attachments {
		if a.Resource.ID == id {
			return true
		}
	}
	return false
}

func validateEnvironmentResources(data environmentFileState) error {
	held := map[string]string{}
	for name, lease := range data.Leases {
		if len(lease.Attachments) == 0 && !lease.RuntimeAbsent {
			continue
		}
		if data.Version < 16 || data.Version > environmentStateVersion || name != lease.EnvironmentID || !core.ValidEnvironmentInstanceID(lease.InstanceID) || !core.ValidEnvironmentAttachments(lease.Attachments) {
			return core.ErrIncompatibleState
		}
		if lease.State != core.WorkspaceLeaseAcquiring && lease.State != core.WorkspaceLeaseActive && lease.State != core.WorkspaceLeaseCleanupRequired {
			return core.ErrIncompatibleState
		}
		if lease.RuntimeAbsent && (len(lease.Attachments) == 0 || lease.State != core.WorkspaceLeaseCleanupRequired) {
			return core.ErrIncompatibleState
		}
		if lease.SnapshotSource != "" || lease.WorkspaceID == "" || lease.SourcePath == "" || lease.Owner == "" || lease.AcquiredAt.IsZero() || (lease.AccessMode != core.WorkspaceReadOnly && lease.AccessMode != core.WorkspaceReadWrite) {
			return core.ErrIncompatibleState
		}
		for otherName, other := range data.Leases {
			if otherName != name && other.InstanceID == lease.InstanceID {
				return core.ErrIncompatibleState
			}
		}
		if lease.State == core.WorkspaceLeaseActive {
			env, ok := data.Environments[name]
			if !ok || !lease.MatchesEnvironment(env) {
				return core.ErrIncompatibleState
			}
		}
		for _, a := range lease.Attachments {
			if !core.ValidEnvironmentResourceRef(a.Resource) || held[a.Resource.ID] != "" || lease.PersistentResource.ID == a.Resource.ID {
				return core.ErrIncompatibleState
			}
			held[a.Resource.ID] = lease.InstanceID
			r, ok := data.PersistentResources[a.Resource.ID]
			if !ok {
				if lease.RuntimeAbsent {
					continue
				}
				return core.ErrIncompatibleState
			}
			if r.Ref() != a.Resource || r.EnvironmentInstance != lease.InstanceID || r.Kind != a.Origin.Kind {
				return core.ErrIncompatibleState
			}
			if (r.State == "planned" || r.State == "creating") && r.CopySource != a.Origin.Current {
				return core.ErrIncompatibleState
			}
			if (r.State == "planned" && r.CopyCompleted) || (r.State == "created" && a.Origin.Current != (core.PersistentResourceRef{})) {
				return core.ErrIncompatibleState
			}

			if lease.State == core.WorkspaceLeaseActive && r.State != "ready" {
				return core.ErrIncompatibleState
			}
			if r.State == "deleting" && !lease.RuntimeAbsent {
				return core.ErrIncompatibleState
			}
		}
	}
	for name, env := range data.Environments {
		lease, ok := data.Leases[name]
		if len(env.Attachments) == 0 && len(lease.Attachments) == 0 {
			continue
		}
		binding := lease
		binding.State = core.WorkspaceLeaseActive
		binding.RuntimeAbsent = false
		if data.Version < 16 || !ok || lease.State == core.WorkspaceLeaseAcquiring || !binding.MatchesEnvironment(env) || !slices.Equal(env.Attachments, lease.Attachments) {
			return core.ErrIncompatibleState
		}
	}
	for id, r := range data.PersistentResources {
		if r.EnvironmentInstance == "" {
			if strings.HasPrefix(id, "env-data:") {
				return core.ErrIncompatibleState
			}
			continue
		}
		if data.Version < 16 || data.Version > environmentStateVersion || !core.ValidEnvironmentInstanceID(r.EnvironmentInstance) || !core.ValidEnvironmentResourceRef(r.Ref()) || held[id] != r.EnvironmentInstance || r.SourceOnly || r.WorkspaceID != "" || r.RestoreSource != "" {
			return core.ErrIncompatibleState
		}
		for otherID, other := range data.PersistentResources {
			if id != otherID && (r.Owner == other.Owner || r.NativeRef == other.NativeRef) {
				return core.ErrIncompatibleState
			}
		}
	}
	return nil
}
