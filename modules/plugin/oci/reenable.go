package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type ReenableReport struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
	Removed   bool   `json:"removed"`
}

func (s *Service) IsImageDeleted(ctx context.Context, reference, digest string) (bool, error) {
	if s == nil || s.store == nil {
		return false, core.ErrRuntimeUnavailable
	}
	if err := validateReference(reference); err != nil {
		return false, err
	}
	digest = strings.ToLower(strings.TrimSpace(digest))
	if !digestPattern.MatchString(digest) {
		return false, fmt.Errorf("invalid OCI digest %q: %w", digest, core.ErrInvalidArgument)
	}
	deletions, err := s.store.ListDeletions(ctx)
	if err != nil {
		return false, err
	}
	for _, deletion := range deletions {
		if deletion.Reference == reference && deletion.Digest == digest {
			return true, nil
		}
	}
	return false, nil
}

// ReenableImage removes only the exact immutable deletion tombstone. It does
// not pull the image, mutate Environments, or bypass a later deletion. Requiring
// reference@sha256:... keeps a mutable tag from accidentally re-enabling a
// different identity after the tag moves.
func (s *Service) ReenableImage(ctx context.Context, raw string) (ReenableReport, error) {
	if s == nil || s.store == nil {
		return ReenableReport{}, core.ErrRuntimeUnavailable
	}
	target, err := parseExactImageIdentity(raw)
	if err != nil {
		return ReenableReport{}, err
	}
	removed, err := s.store.RemoveDeletion(ctx, target.Reference, target.Digest)
	if err != nil {
		return ReenableReport{}, err
	}
	return ReenableReport{Reference: target.Reference, Digest: target.Digest, Removed: removed}, nil
}

func parseExactImageIdentity(raw string) (deleteTarget, error) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return deleteTarget{}, core.ErrInvalidArgument
	}
	cut := strings.LastIndexByte(raw, '@')
	if cut <= 0 || cut == len(raw)-1 {
		return deleteTarget{}, fmt.Errorf("OCI identity must use reference@sha256:...: %w", core.ErrInvalidArgument)
	}
	reference := raw[:cut]
	digest := strings.ToLower(raw[cut+1:])
	if err := validateReference(reference); err != nil {
		return deleteTarget{}, err
	}
	if !digestPattern.MatchString(digest) {
		return deleteTarget{}, fmt.Errorf("invalid OCI digest %q: %w", digest, core.ErrInvalidArgument)
	}
	return deleteTarget{Reference: reference, Digest: digest}, nil
}

func (s *Store) RemoveDeletion(_ context.Context, reference, digest string) (bool, error) {
	if s == nil {
		return false, core.ErrInvalidArgument
	}
	if err := validateReference(reference); err != nil {
		return false, err
	}
	digest = strings.ToLower(strings.TrimSpace(digest))
	if !digestPattern.MatchString(digest) {
		return false, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockStateFile(s.path)
	if err != nil {
		return false, err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return false, err
	}
	key := (Deletion{Reference: reference, Digest: digest}).Key()
	if _, ok := state.Deletions[key]; !ok {
		return false, nil
	}
	delete(state.Deletions, key)
	if err := s.write(state); err != nil {
		return false, err
	}
	return true, nil
}
