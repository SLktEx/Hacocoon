package persistentresource

import (
	"context"
	"errors"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// RecoverEnvironmentGeneration resumes a durably completed copy, never the copy
// request itself. Exact producer/origin receipts restrict this operation to the
// ordinary Environment publication contract. Unknown outcomes remain owned.
func (s *Service) RecoverEnvironmentGeneration(ctx context.Context, ref core.PersistentResourceRef) (core.ResourceGenerationPublication, error) {
	result := core.ResourceGenerationPublication{State: "recovery-required"}
	if s == nil || s.Store == nil || s.Backend == nil {
		return result, core.ErrUnsupported
	}
	catalog, ok := s.Store.(generationStore)
	if !ok {
		return result, core.ErrUnsupported
	}
	target, err := s.Store.GetPersistentResource(ctx, ref.ID)
	if err != nil {
		return result, err
	}
	if target.Ref() != ref {
		return result, core.ErrCapabilityStale
	}
	if !core.ValidGenerationResource(ref) || !target.SourceOnly || target.EnvironmentInstance != "" || target.WorkspaceID != "" || !core.ValidResourceGeneration(target.PublicationOrigin) || !core.ValidEnvironmentResourceRef(target.Producer) || target.Kind != target.PublicationOrigin.Kind {
		return result, core.ErrIncompatibleState
	}
	result.Candidate = target
	if target.State == "creating" {
		if !target.CopyCompleted || target.CopySource != target.Producer {
			return result, core.ErrRecoveryRequired
		}
		target, err = s.RecoverCopy(ctx, target.Ref())
		result.Candidate = target
		if err != nil {
			return result, errors.Join(err, core.ErrRecoveryRequired)
		}
	}
	if target.State != "ready" || target.CopySource != (core.PersistentResourceRef{}) || target.CopyCompleted {
		return result, core.ErrRecoveryRequired
	}
	current, err := catalog.GetResourceGeneration(ctx, target.PublicationOrigin.Name)
	if err != nil {
		return result, err
	}
	result.Generation = current
	if current.Current == target.Ref() {
		result.State = "published"
		return result, nil
	}
	if current != target.PublicationOrigin {
		result.State = "retained"
		return result, nil
	}
	// Reuse the normal atomic adoption boundary, without treating a refused old
	// candidate as newly failed publication cleanup.
	return s.selectGeneration(ctx, target.PublicationOrigin, result, false)
}
