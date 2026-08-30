package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const seedSelectionStateVersion = 1

// SeedPin is an explicit operator request to include one immutable OCI image in
// future Seed builds even when it falls outside automatic promotion ranking.
type SeedPin struct {
	Reference string    `json:"reference"`
	Digest    string    `json:"digest"`
	PinnedAt  time.Time `json:"pinned_at"`
}

func (p SeedPin) Key() string { return p.Reference + "@" + p.Digest }

// SeedReenable records an explicit operator decision that supersedes an older
// deletion tombstone for one immutable image identity. A later deletion wins
// again, so re-enable never erases deletion history.
type SeedReenable struct {
	Reference   string    `json:"reference"`
	Digest      string    `json:"digest"`
	ReenabledAt time.Time `json:"re_enabled_at"`
}

func (r SeedReenable) Key() string { return r.Reference + "@" + r.Digest }

// SeedSelection is the operator-facing effective state for one immutable image.
type SeedSelection struct {
	Reference   string     `json:"reference"`
	Digest      string     `json:"digest"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	Reenabled   bool       `json:"re_enabled"`
	ReenabledAt *time.Time `json:"re_enabled_at,omitempty"`
	Deleted     bool       `json:"deleted"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type seedSelectionFileState struct {
	Version   int                     `json:"version"`
	Pins      map[string]SeedPin      `json:"pins,omitempty"`
	Reenables map[string]SeedReenable `json:"re_enables,omitempty"`
}

type seedSelectionPolicy struct {
	blocked   map[string]struct{}
	reenabled map[string]struct{}
	pins      map[string]SeedPin
}

func (p seedSelectionPolicy) isBlocked(key string) bool {
	_, ok := p.blocked[key]
	return ok
}

func (p seedSelectionPolicy) isReenabled(key string) bool {
	_, ok := p.reenabled[key]
	return ok
}

func (s *Service) PinSeedImage(ctx context.Context, raw string, reenable bool) (SeedSelection, error) {
	if s == nil || s.store == nil {
		return SeedSelection{}, core.ErrRuntimeUnavailable
	}
	identity, err := parseImmutableImageIdentity(raw)
	if err != nil {
		return SeedSelection{}, err
	}
	before, err := s.seedSelection(ctx, identity)
	if err != nil {
		return SeedSelection{}, err
	}
	if before.Deleted && !reenable {
		return before, fmt.Errorf("OCI Seed image %q is deleted; pinning it requires explicit --re-enable: %w", identity.Key(), core.ErrInvalidArgument)
	}
	if reenable && before.DeletedAt == nil {
		return before, fmt.Errorf("OCI Seed image %q has no deletion tombstone to re-enable: %w", identity.Key(), core.ErrNotFound)
	}
	now := s.now().UTC()
	pin := SeedPin{Reference: identity.Reference, Digest: identity.Digest, PinnedAt: now}
	var override *SeedReenable
	if reenable {
		override = &SeedReenable{Reference: identity.Reference, Digest: identity.Digest, ReenabledAt: now}
	}
	if err := s.store.putSeedSelection(ctx, pin, override); err != nil {
		return SeedSelection{}, err
	}
	after, err := s.seedSelection(ctx, identity)
	if err != nil {
		return SeedSelection{}, err
	}
	if after.Deleted {
		return after, fmt.Errorf("OCI Seed image %q was deleted concurrently with pin/re-enable; deletion remains effective: %w", identity.Key(), core.ErrIncompatibleState)
	}
	return after, nil
}

func (s *Service) UnpinSeedImage(ctx context.Context, raw string) (SeedSelection, error) {
	if s == nil || s.store == nil {
		return SeedSelection{}, core.ErrRuntimeUnavailable
	}
	identity, err := parseImmutableImageIdentity(raw)
	if err != nil {
		return SeedSelection{}, err
	}
	if err := s.store.deleteSeedPin(ctx, identity.Key()); err != nil {
		return SeedSelection{}, err
	}
	return s.seedSelection(ctx, identity)
}

func (s *Service) ReenableSeedImage(ctx context.Context, raw string) (SeedSelection, error) {
	if s == nil || s.store == nil {
		return SeedSelection{}, core.ErrRuntimeUnavailable
	}
	identity, err := parseImmutableImageIdentity(raw)
	if err != nil {
		return SeedSelection{}, err
	}
	before, err := s.seedSelection(ctx, identity)
	if err != nil {
		return SeedSelection{}, err
	}
	if before.DeletedAt == nil {
		return before, fmt.Errorf("OCI Seed image %q has no deletion tombstone to re-enable: %w", identity.Key(), core.ErrNotFound)
	}
	now := s.now().UTC()
	if !now.After(*before.DeletedAt) {
		return before, fmt.Errorf("current time %s does not follow deletion time %s for %q: %w", now.Format(time.RFC3339Nano), before.DeletedAt.Format(time.RFC3339Nano), identity.Key(), core.ErrIncompatibleState)
	}
	if err := s.store.putSeedReenable(ctx, SeedReenable{Reference: identity.Reference, Digest: identity.Digest, ReenabledAt: now}); err != nil {
		return SeedSelection{}, err
	}
	return s.seedSelection(ctx, identity)
}

func (s *Service) seedSelection(ctx context.Context, identity immutableImageIdentity) (SeedSelection, error) {
	// Read permissive operator state first and destructive tombstones last. If a
	// concurrent mutation lands between the two files, a new deletion is seen
	// immediately while a new re-enable is deferred to the next read. That makes
	// the cross-file snapshot fail closed without pretending the files are one
	// atomic transaction.
	state, err := s.store.readSeedSelectionsLocked()
	if err != nil {
		return SeedSelection{}, err
	}
	deletions, err := s.store.ListDeletions(ctx)
	if err != nil {
		return SeedSelection{}, err
	}
	selection := SeedSelection{Reference: identity.Reference, Digest: identity.Digest}
	for _, deletion := range deletions {
		if deletion.Key() == identity.Key() {
			t := deletion.DeletedAt
			selection.DeletedAt = &t
			break
		}
	}
	if pin, ok := state.Pins[identity.Key()]; ok {
		t := pin.PinnedAt
		selection.Pinned = true
		selection.PinnedAt = &t
	}
	if override, ok := state.Reenables[identity.Key()]; ok {
		t := override.ReenabledAt
		selection.ReenabledAt = &t
		if selection.DeletedAt != nil && t.After(*selection.DeletedAt) {
			selection.Reenabled = true
		}
	}
	selection.Deleted = selection.DeletedAt != nil && !selection.Reenabled
	return selection, nil
}

func (s *Service) seedSelectionPolicy(ctx context.Context) (seedSelectionPolicy, error) {
	// As above, read allow-like state first and deny-like state last so a
	// concurrent cross-file update can only make this snapshot stricter.
	state, err := s.store.readSeedSelectionsLocked()
	if err != nil {
		return seedSelectionPolicy{}, err
	}
	deletions, err := s.store.ListDeletions(ctx)
	if err != nil {
		return seedSelectionPolicy{}, err
	}
	policy := seedSelectionPolicy{
		blocked:   map[string]struct{}{},
		reenabled: map[string]struct{}{},
		pins:      make(map[string]SeedPin, len(state.Pins)),
	}
	for key, pin := range state.Pins {
		policy.pins[key] = pin
	}
	for _, deletion := range deletions {
		if override, ok := state.Reenables[deletion.Key()]; ok && override.ReenabledAt.After(deletion.DeletedAt) {
			policy.reenabled[deletion.Key()] = struct{}{}
			continue
		}
		policy.blocked[deletion.Key()] = struct{}{}
	}
	return policy, nil
}

func applySeedPins(recommendations []Recommendation, policy seedSelectionPolicy) []Recommendation {
	byKey := make(map[string]int, len(recommendations))
	for i := range recommendations {
		key := recommendations[i].Reference + "@" + recommendations[i].Digest
		byKey[key] = i
		if pin, ok := policy.pins[key]; ok {
			t := pin.PinnedAt
			recommendations[i].Pinned = true
			recommendations[i].PinnedAt = &t
		}
		if policy.isReenabled(key) {
			recommendations[i].Reenabled = true
		}
	}
	for key, pin := range policy.pins {
		if policy.isBlocked(key) {
			continue
		}
		if _, ok := byKey[key]; ok {
			continue
		}
		t := pin.PinnedAt
		recommendations = append(recommendations, Recommendation{
			Reference: pin.Reference,
			Digest:    pin.Digest,
			Pinned:    true,
			PinnedAt:  &t,
			Reenabled: policy.isReenabled(key),
		})
	}
	return recommendations
}

type immutableImageIdentity struct {
	Reference string
	Digest    string
}

func (i immutableImageIdentity) Key() string { return i.Reference + "@" + i.Digest }

func parseImmutableImageIdentity(raw string) (immutableImageIdentity, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || hasControl(raw) {
		return immutableImageIdentity{}, fmt.Errorf("invalid immutable OCI image identity %q: %w", raw, core.ErrInvalidArgument)
	}
	cut := strings.LastIndexByte(raw, '@')
	if cut <= 0 || cut == len(raw)-1 {
		return immutableImageIdentity{}, fmt.Errorf("OCI Seed selection requires reference@sha256:... identity %q: %w", raw, core.ErrInvalidArgument)
	}
	reference := raw[:cut]
	digest := strings.ToLower(raw[cut+1:])
	if err := validateReference(reference); err != nil {
		return immutableImageIdentity{}, err
	}
	if !digestPattern.MatchString(digest) {
		return immutableImageIdentity{}, fmt.Errorf("invalid OCI digest %q: %w", digest, core.ErrInvalidArgument)
	}
	return immutableImageIdentity{Reference: reference, Digest: digest}, nil
}

func (s *Store) selectionPath() string {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.path), "oci-selections.json")
}

func (s *Store) putSeedSelection(_ context.Context, pin SeedPin, reenable *SeedReenable) error {
	if s == nil {
		return core.ErrInvalidArgument
	}
	if err := validateSeedPin(pin); err != nil {
		return err
	}
	if reenable != nil {
		if err := validateSeedReenable(*reenable); err != nil {
			return err
		}
		if reenable.Key() != pin.Key() {
			return fmt.Errorf("Seed pin and re-enable identities differ: %w", core.ErrInvalidArgument)
		}
	}
	return s.mutateSeedSelections(func(state *seedSelectionFileState) error {
		state.Pins[pin.Key()] = pin
		if reenable != nil {
			state.Reenables[reenable.Key()] = *reenable
		}
		return nil
	})
}

func (s *Store) deleteSeedPin(_ context.Context, key string) error {
	if s == nil || strings.TrimSpace(key) == "" {
		return core.ErrInvalidArgument
	}
	return s.mutateSeedSelections(func(state *seedSelectionFileState) error {
		delete(state.Pins, key)
		return nil
	})
}

func (s *Store) putSeedReenable(_ context.Context, override SeedReenable) error {
	if s == nil {
		return core.ErrInvalidArgument
	}
	if err := validateSeedReenable(override); err != nil {
		return err
	}
	return s.mutateSeedSelections(func(state *seedSelectionFileState) error {
		state.Reenables[override.Key()] = override
		return nil
	})
}

func (s *Store) listSeedPins(context.Context) ([]SeedPin, error) {
	if s == nil {
		return nil, core.ErrInvalidArgument
	}
	state, err := s.readSeedSelectionsLocked()
	if err != nil {
		return nil, err
	}
	result := make([]SeedPin, 0, len(state.Pins))
	for _, pin := range state.Pins {
		result = append(result, pin)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result, nil
}

func (s *Store) listSeedReenables(context.Context) ([]SeedReenable, error) {
	if s == nil {
		return nil, core.ErrInvalidArgument
	}
	state, err := s.readSeedSelectionsLocked()
	if err != nil {
		return nil, err
	}
	result := make([]SeedReenable, 0, len(state.Reenables))
	for _, override := range state.Reenables {
		result = append(result, override)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result, nil
}

func (s *Store) readSeedSelectionsLocked() (seedSelectionFileState, error) {
	if s == nil {
		return seedSelectionFileState{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.selectionPath()
	if path == "" {
		return seedSelectionFileState{}, core.ErrInvalidArgument
	}
	unlock, err := lockStateFile(path)
	if err != nil {
		return seedSelectionFileState{}, err
	}
	defer unlock()
	return readSeedSelectionFile(path)
}

func (s *Store) mutateSeedSelections(mutate func(*seedSelectionFileState) error) error {
	if s == nil || mutate == nil {
		return core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.selectionPath()
	if path == "" {
		return core.ErrInvalidArgument
	}
	unlock, err := lockStateFile(path)
	if err != nil {
		return err
	}
	defer unlock()
	state, err := readSeedSelectionFile(path)
	if err != nil {
		return err
	}
	if err := mutate(&state); err != nil {
		return err
	}
	return writeSeedSelectionFile(path, state)
}

func readSeedSelectionFile(path string) (seedSelectionFileState, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return seedSelectionFileState{Version: seedSelectionStateVersion, Pins: map[string]SeedPin{}, Reenables: map[string]SeedReenable{}}, nil
	}
	if err != nil {
		return seedSelectionFileState{}, fmt.Errorf("read OCI Seed selection state: %w", err)
	}
	var state seedSelectionFileState
	if err := json.Unmarshal(contents, &state); err != nil {
		return seedSelectionFileState{}, fmt.Errorf("decode OCI Seed selection state: %w", err)
	}
	if state.Version != seedSelectionStateVersion {
		return seedSelectionFileState{}, fmt.Errorf("OCI Seed selection state version %d is unsupported: %w", state.Version, core.ErrIncompatibleState)
	}
	if state.Pins == nil {
		state.Pins = map[string]SeedPin{}
	}
	if state.Reenables == nil {
		state.Reenables = map[string]SeedReenable{}
	}
	for key, pin := range state.Pins {
		if err := validateSeedPin(pin); err != nil {
			return seedSelectionFileState{}, err
		}
		if key != pin.Key() {
			return seedSelectionFileState{}, fmt.Errorf("OCI Seed pin key %q does not match identity %q: %w", key, pin.Key(), core.ErrIncompatibleState)
		}
	}
	for key, override := range state.Reenables {
		if err := validateSeedReenable(override); err != nil {
			return seedSelectionFileState{}, err
		}
		if key != override.Key() {
			return seedSelectionFileState{}, fmt.Errorf("OCI Seed re-enable key %q does not match identity %q: %w", key, override.Key(), core.ErrIncompatibleState)
		}
	}
	return state, nil
}

func writeSeedSelectionFile(path string, state seedSelectionFileState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create OCI Seed selection state directory: %w", err)
	}
	state.Version = seedSelectionStateVersion
	if state.Pins == nil {
		state.Pins = map[string]SeedPin{}
	}
	if state.Reenables == nil {
		state.Reenables = map[string]SeedReenable{}
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode OCI Seed selection state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".oci-selections-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary OCI Seed selection state: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary OCI Seed selection state: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temporary OCI Seed selection state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary OCI Seed selection state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary OCI Seed selection state: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit OCI Seed selection state: %w", err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func validateSeedPin(pin SeedPin) error {
	if err := validateReference(pin.Reference); err != nil {
		return err
	}
	if !digestPattern.MatchString(pin.Digest) || pin.PinnedAt.IsZero() {
		return fmt.Errorf("invalid OCI Seed pin identity %q@%q: %w", pin.Reference, pin.Digest, core.ErrInvalidArgument)
	}
	return nil
}

func validateSeedReenable(override SeedReenable) error {
	if err := validateReference(override.Reference); err != nil {
		return err
	}
	if !digestPattern.MatchString(override.Digest) || override.ReenabledAt.IsZero() {
		return fmt.Errorf("invalid OCI Seed re-enable identity %q@%q: %w", override.Reference, override.Digest, core.ErrInvalidArgument)
	}
	return nil
}
