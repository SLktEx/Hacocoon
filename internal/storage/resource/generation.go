package persistentresource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type generationStore interface {
	GetResourceGeneration(context.Context, string) (core.ResourceGeneration, error)
	AdvanceResourceGeneration(context.Context, core.ResourceGeneration, core.PersistentResourceRef) (core.ResourceGeneration, error)
	BeginGenerationResourceDelete(context.Context, core.PersistentResourceRef) (core.PersistentResource, error)
}

// PublishGeneration prepares a whole source through canonical resource creation.
// A stale producer never overwrites current data and never merges file contents.
func (s *Service) PublishGeneration(ctx context.Context, expected core.ResourceGeneration, prepare func(context.Context, core.PersistentResource) error) (result core.ResourceGenerationPublication, err error) {
	if s == nil || s.Store == nil || s.Backend == nil {
		return result, core.ErrUnsupported
	}
	store, ok := s.Store.(generationStore)
	if !ok {
		return result, core.ErrUnsupported
	}
	if !core.ValidResourceGeneration(expected) || prepare == nil {
		return result, core.ErrInvalidArgument
	}
	current, err := store.GetResourceGeneration(ctx, expected.Name)
	if err != nil {
		return result, err
	}
	result.Generation = current
	if current != expected {
		result.State = "skipped"
		return result, nil
	}
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	candidate, err := s.PublishSource(ctx, "generation:"+hex.EncodeToString(nonce[:]), expected.Kind, prepare)
	result.Candidate = candidate
	if err != nil {
		result.State = "failed"
		if candidate.ID != "" {
			result.State = "recovery-required"
			err = errors.Join(err, core.ErrRecoveryRequired)
		}
		return result, err
	}
	return s.adoptGeneration(ctx, expected, result)
}

// Both prepared sources and stopped-Environment copies use one CAS/cleanup path.
func (s *Service) adoptGeneration(ctx context.Context, expected core.ResourceGeneration, result core.ResourceGenerationPublication) (core.ResourceGenerationPublication, error) {
	return s.selectGeneration(ctx, expected, result, true)
}

func (s *Service) selectGeneration(ctx context.Context, expected core.ResourceGeneration, result core.ResourceGenerationPublication, cleanupRefused bool) (core.ResourceGenerationPublication, error) {
	store, ok := s.Store.(generationStore)
	if !ok {
		return result, core.ErrUnsupported
	}
	candidate := result.Candidate
	adopted, err := store.AdvanceResourceGeneration(ctx, expected, candidate.Ref())
	if err == nil {
		result.Generation, result.State = adopted, "published"
		return result, nil
	}
	if !errors.Is(err, core.ErrSourceGenerationStale) {
		// Persistence may be ambiguous. Never delete a candidate on this path.
		result.State = "recovery-required"
		return result, errors.Join(err, core.ErrRecoveryRequired)
	}
	if !cleanupRefused {
		// Recovery may revisit a previously selected candidate after another
		// publisher or reset. It cannot prove that such data was never selected.
		current, readErr := store.GetResourceGeneration(ctx, expected.Name)
		result.Generation, result.State = current, "retained"
		if readErr != nil {
			result.State = "recovery-required"
			return result, errors.Join(readErr, core.ErrRecoveryRequired)
		}
		if current.Current == candidate.Ref() {
			result.State = "published"
		}
		return result, nil
	}
	// CAS refusal proves that this candidate was not adopted. Delete only its
	// exact ready owner through the common backend/absence/finalization sequence.
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	deleting, cleanupErr := store.BeginGenerationResourceDelete(cleanup, candidate.Ref())
	if cleanupErr == nil {
		cleanupErr = s.finishDelete(cleanup, deleting)
	}
	if cleanupErr != nil {
		result.State = "cleanup-required"
		return result, errors.Join(cleanupErr, core.ErrRecoveryRequired)
	}
	result.Candidate = core.PersistentResource{}
	current, err := store.GetResourceGeneration(ctx, expected.Name)
	if err == nil {
		result.Generation = current
	}
	result.State = "skipped"
	return result, err
}

// DeleteUnselectedGeneration uses the same exact-owner deletion path as CAS
// refusal. Current selections and unfinished copies remain protected by state.
func (s *Service) DeleteUnselectedGeneration(ctx context.Context, ref core.PersistentResourceRef) error {
	store, ok := s.Store.(generationStore)
	if !ok {
		return core.ErrUnsupported
	}
	deleting, err := store.BeginGenerationResourceDelete(ctx, ref)
	if err != nil {
		return err
	}
	return s.finishDelete(ctx, deleting)
}
