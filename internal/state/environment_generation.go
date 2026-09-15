package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// BeginEnvironmentGenerationCopy pins a child of the exact active lease. The
// canonical Environment operation holds its lifecycle locks and proves stopped
// runtime ownership; the provider independently verifies the stopped consumer.
func (s *EnvironmentJSONStore) BeginEnvironmentGenerationCopy(ctx context.Context, expected core.WorkspaceLease, source, target core.PersistentResource) error {
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		lease, ok := data.Leases[expected.EnvironmentID]
		if !ok || !lease.Equal(expected) || !environmentGenerationCopy(*data, source, target, lease) {
			return false, core.ErrCapabilityStale
		}
		if snapshotBusy(*data, lease.EnvironmentID) || environmentResourcesBusy(*data, lease.EnvironmentID) {
			return false, core.ErrRecoveryRequired
		}
		if err := reservePersistentResourceCopy(data, source, target); err != nil {
			return false, err
		}
		return true, nil
	})
}

func environmentGenerationCopy(data environmentFileState, source, target core.PersistentResource, lease core.WorkspaceLease) bool {
	if !core.ValidEnvironmentInstanceID(source.EnvironmentInstance) || !core.ValidEnvironmentResourceRef(source.Ref()) || source.SourceOnly || source.WorkspaceID != "" || source.State != "ready" || source.EnvironmentInstance != lease.InstanceID || lease.State != core.WorkspaceLeaseActive || lease.RuntimeAbsent || !lease.MatchesEnvironment(data.Environments[lease.EnvironmentID]) {
		return false
	}
	if !core.ValidGenerationResource(target.Ref()) || !target.SourceOnly || target.EnvironmentInstance != "" || target.WorkspaceID != "" || target.RestoreSource != "" || target.State != "creating" || target.CopySource != source.Ref() || target.Kind != source.Kind {
		return false
	}
	for _, area := range lease.Attachments {
		if area.Resource == source.Ref() && area.Origin.Kind == source.Kind && target.PublicationOrigin == area.Origin && target.Producer == source.Ref() {
			return true
		}
	}
	return false
}

func environmentResourcesBusy(data environmentFileState, name string) bool {
	for _, area := range data.Leases[name].Attachments {
		if data.PersistentResources[area.Resource.ID].State == "clearing" || persistentCopyReserved(data, area.Resource.ID) {
			return true
		}
	}
	return false
}

// Incomplete data maintenance survives process exit and fences start/access/deletion.
func (s *EnvironmentJSONStore) CheckEnvironmentResourcesIdle(ctx context.Context, name string) error {
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		if environmentResourcesBusy(*data, name) {
			return false, core.ErrRecoveryRequired
		}
		return false, nil
	})
}
