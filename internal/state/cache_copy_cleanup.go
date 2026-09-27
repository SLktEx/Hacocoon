package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func validCacheCopyReceipt(r core.PersistentResource) bool {
	return core.ValidGenerationResource(r.Ref()) && r.SourceOnly && r.EnvironmentInstance == "" && r.WorkspaceID == "" && r.RestoreSource == "" && core.ValidResourceGeneration(r.PublicationOrigin) && r.Kind == "build-cache" && r.PublicationOrigin.Kind == r.Kind && core.ValidEnvironmentResourceRef(r.Producer) && r.CopySource == r.Producer && r.CopyOperation != "" && len(r.CopyOperation) <= 256 && (r.State == "creating" || (r.State == "deleting" && r.CopyCleanup))
}

// RecordCacheCopyOperation binds the submitted provider request to the already
// reserved destination. This receipt cannot be replaced by a later request.
func (s *EnvironmentJSONStore) RecordCacheCopyOperation(_ context.Context, r core.PersistentResource, operation string) (core.PersistentResource, error) {
	updated := r
	updated.CopyOperation = operation
	if r.CopyOperation != "" || r.CopyCompleted || r.CopyCleanup || !validCacheCopyReceipt(updated) {
		return r, core.ErrInvalidArgument
	}
	err := s.resourceTransaction(func(d *environmentFileState) error {
		if d.PersistentResources[r.ID] != r {
			return core.ErrCapabilityStale
		}
		d.PersistentResources[r.ID] = updated
		return nil
	})
	if err != nil {
		return r, err
	}
	return updated, nil
}

// MarkCacheCopyCleanup excludes completion/publication before waiting for the
// provider. Reservations survive both process exit and ambiguous deletion.
func (s *EnvironmentJSONStore) MarkCacheCopyCleanup(_ context.Context, r core.PersistentResource) (core.PersistentResource, error) {
	if !validCacheCopyReceipt(r) || r.CopyCompleted {
		return r, core.ErrRecoveryRequired
	}
	updated := r
	updated.CopyCleanup = true
	err := s.resourceTransaction(func(d *environmentFileState) error {
		if d.PersistentResources[r.ID] != r {
			return core.ErrCapabilityStale
		}
		d.PersistentResources[r.ID] = updated
		return nil
	})
	if err != nil {
		return r, err
	}
	return updated, nil
}

// BeginStoppedCacheCopyDelete is called only after the backend has positively
// observed the saved operation terminate. Reuse all ordinary deletion fences.
func (s *EnvironmentJSONStore) BeginStoppedCacheCopyDelete(ctx context.Context, r core.PersistentResource) (core.PersistentResource, error) {
	if !validCacheCopyReceipt(r) || !r.CopyCleanup || r.CopyCompleted {
		return r, core.ErrRecoveryRequired
	}
	return s.beginPersistentResourceDelete(ctx, r.ID, persistentResourceDeletion{Identity: r.Ref(), StoppedCopy: r})
}
