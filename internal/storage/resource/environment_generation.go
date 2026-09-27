package persistentresource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"time"
)

// PublishEnvironmentGeneration copies only an enrolled area, under its parent's
// canonical stopped lifecycle guard. The immutable origin supplies the CAS token.
func (s *Service) PublishEnvironmentGeneration(ctx context.Context, lease core.WorkspaceLease, area core.EnvironmentAttachment) (result core.ResourceGenerationPublication, err error) {
	if s == nil || s.Store == nil || s.Backend == nil {
		return result, core.ErrUnsupported
	}
	store, ok := s.Store.(interface {
		generationStore
		BeginEnvironmentGenerationCopy(context.Context, core.WorkspaceLease, core.PersistentResource, core.PersistentResource) error
	})
	if !ok {
		return result, core.ErrUnsupported
	}
	matched := false
	for _, a := range lease.Attachments {
		if a == area {
			matched = true
		}
	}
	if !matched || !core.ValidResourceGeneration(area.Origin) {
		return result, core.ErrInvalidArgument
	}
	current, err := store.GetResourceGeneration(ctx, area.Origin.Name)
	if err != nil {
		return result, err
	}
	result.Generation = current
	if current != area.Origin {
		result.State = "skipped"
		return result, nil
	}
	source, err := s.Store.GetPersistentResource(ctx, area.Resource.ID)
	if err != nil {
		return result, err
	}
	if source.Ref() != area.Resource || source.EnvironmentInstance != lease.InstanceID {
		return result, core.ErrCapabilityStale
	}
	var nonce, owner [16]byte
	_, _ = rand.Read(nonce[:])
	_, _ = rand.Read(owner[:])
	target := core.PersistentResource{ID: "generation:" + hex.EncodeToString(nonce[:]), Owner: hex.EncodeToString(owner[:]), Kind: area.Origin.Kind, SourceOnly: true, State: "creating", CreatedAt: time.Now().UTC(), CopySource: source.Ref(), PublicationOrigin: area.Origin, Producer: source.Ref()}
	target.NativeRef, err = s.Backend.Plan(ctx, target.Kind, target.Owner)
	if err != nil {
		return result, err
	}
	if err = store.BeginEnvironmentGenerationCopy(ctx, lease, source, target); err != nil {
		return result, err
	}
	result.Candidate, err = s.copyReserved(ctx, source, target)
	if err != nil {
		result.State = "recovery-required"
		// Re-read durable state: a failed receipt write may actually have
		// committed. Never turn a completed copy into failed-copy cleanup.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		held, readErr := s.Store.GetPersistentResource(cleanupCtx, target.ID)
		if readErr == nil && held.Ref() == target.Ref() && held.CopyOperation != "" && !held.CopyCompleted {
			cleaned, cleanupErr := s.cleanupCacheCopy(cleanupCtx, held)
			result.Candidate = cleaned
			if cleanupErr == nil {
				result.State = "cleaned"
				return result, errors.New("cache copy failed; unfinished destination removed; source retained")
			}
			err = errors.Join(err, cleanupErr)
		}
		return result, errors.Join(err, core.ErrRecoveryRequired)
	}
	return s.adoptGeneration(ctx, area.Origin, result)
}
