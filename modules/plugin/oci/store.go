package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	legacyUsageStateVersion = 1
	usageStateVersion       = 2
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type Image struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag,omitempty"`
	Digest     string `json:"digest,omitempty"`
}

func (i Image) Reference() string {
	if i.Tag == "" {
		return i.Repository
	}
	return i.Repository + ":" + i.Tag
}

type Snapshot struct {
	Environment string    `json:"environment"`
	SampledAt   time.Time `json:"sampled_at"`
	Images      []Image   `json:"images"`
}

// Deletion is a plugin-owned trusted-side tombstone for one immutable OCI
// image identity. It prevents stale observations from immediately undoing an
// explicit Seed-selection deletion decision.
type Deletion struct {
	Reference string    `json:"reference"`
	Digest    string    `json:"digest"`
	DeletedAt time.Time `json:"deleted_at"`
}

func (d Deletion) Key() string { return d.Reference + "@" + d.Digest }

type usageFileState struct {
	Version   int                 `json:"version"`
	Snapshots map[string]Snapshot `json:"snapshots"`
	Deletions map[string]Deletion `json:"deletions,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Get(_ context.Context, environment string) (Snapshot, error) {
	if s == nil || strings.TrimSpace(environment) == "" {
		return Snapshot{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockStateFile(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, ok := state.Snapshots[environment]
	if !ok {
		return Snapshot{}, fmt.Errorf("OCI plugin usage snapshot %q: %w", environment, core.ErrNotFound)
	}
	return snapshot, nil
}

func (s *Store) List(context.Context) ([]Snapshot, error) {
	if s == nil {
		return nil, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockStateFile(s.path)
	if err != nil {
		return nil, err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return nil, err
	}
	result := make([]Snapshot, 0, len(state.Snapshots))
	for _, snapshot := range state.Snapshots {
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Environment < result[j].Environment })
	return result, nil
}

func (s *Store) Put(_ context.Context, snapshot Snapshot) error {
	if s == nil {
		return core.ErrInvalidArgument
	}
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockStateFile(s.path)
	if err != nil {
		return err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return err
	}
	state.Snapshots[snapshot.Environment] = snapshot
	return s.write(state)
}

func (s *Store) PutDeletion(_ context.Context, deletion Deletion) error {
	if s == nil {
		return core.ErrInvalidArgument
	}
	if err := validateDeletion(deletion); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockStateFile(s.path)
	if err != nil {
		return err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return err
	}
	state.Deletions[deletion.Key()] = deletion
	return s.write(state)
}

func (s *Store) ListDeletions(context.Context) ([]Deletion, error) {
	if s == nil {
		return nil, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockStateFile(s.path)
	if err != nil {
		return nil, err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return nil, err
	}
	result := make([]Deletion, 0, len(state.Deletions))
	for _, deletion := range state.Deletions {
		result = append(result, deletion)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].DeletedAt.Equal(result[j].DeletedAt) {
			return result[i].DeletedAt.Before(result[j].DeletedAt)
		}
		return result[i].Key() < result[j].Key()
	})
	return result, nil
}

func (s *Store) read() (usageFileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return usageFileState{Version: usageStateVersion, Snapshots: map[string]Snapshot{}, Deletions: map[string]Deletion{}}, nil
	}
	if err != nil {
		return usageFileState{}, fmt.Errorf("read OCI plugin usage state: %w", err)
	}
	var state usageFileState
	if err := json.Unmarshal(contents, &state); err != nil {
		return usageFileState{}, fmt.Errorf("decode OCI plugin usage state: %w", err)
	}
	switch state.Version {
	case legacyUsageStateVersion, usageStateVersion:
	default:
		return usageFileState{}, fmt.Errorf("OCI plugin usage state version %d is unsupported: %w", state.Version, core.ErrIncompatibleState)
	}
	if state.Snapshots == nil {
		state.Snapshots = map[string]Snapshot{}
	}
	if state.Deletions == nil {
		state.Deletions = map[string]Deletion{}
	}
	for environment, snapshot := range state.Snapshots {
		if snapshot.Environment == "" {
			snapshot.Environment = environment
		}
		if err := validateSnapshot(snapshot); err != nil {
			return usageFileState{}, err
		}
		state.Snapshots[environment] = snapshot
	}
	for key, deletion := range state.Deletions {
		if err := validateDeletion(deletion); err != nil {
			return usageFileState{}, err
		}
		if key != deletion.Key() {
			return usageFileState{}, fmt.Errorf("OCI plugin deletion key %q does not match identity %q: %w", key, deletion.Key(), core.ErrIncompatibleState)
		}
	}
	state.Version = usageStateVersion
	return state, nil
}

func (s *Store) write(state usageFileState) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create OCI plugin usage state directory: %w", err)
	}
	state.Version = usageStateVersion
	if state.Snapshots == nil {
		state.Snapshots = map[string]Snapshot{}
	}
	if state.Deletions == nil {
		state.Deletions = map[string]Deletion{}
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode OCI plugin usage state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".oci-usage-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary OCI plugin usage state: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary OCI plugin usage state: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temporary OCI plugin usage state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary OCI plugin usage state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary OCI plugin usage state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit OCI plugin usage state: %w", err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func validateSnapshot(snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.Environment) == "" || snapshot.SampledAt.IsZero() {
		return core.ErrInvalidArgument
	}
	for _, image := range snapshot.Images {
		if err := validateImage(image); err != nil {
			return err
		}
	}
	return nil
}

func validateDeletion(deletion Deletion) error {
	if err := validateReference(deletion.Reference); err != nil {
		return err
	}
	if !digestPattern.MatchString(deletion.Digest) || deletion.DeletedAt.IsZero() {
		return fmt.Errorf("invalid OCI plugin deletion identity %q@%q: %w", deletion.Reference, deletion.Digest, core.ErrInvalidArgument)
	}
	return nil
}

func validateReference(reference string) error {
	if reference == "" || strings.TrimSpace(reference) != reference || hasControl(reference) || strings.ContainsAny(reference, "\t\r\n") || strings.Contains(reference, "@") {
		return fmt.Errorf("invalid OCI image reference %q: %w", reference, core.ErrInvalidArgument)
	}
	return nil
}

func validateImage(image Image) error {
	if image.Repository == "" || strings.TrimSpace(image.Repository) != image.Repository || hasControl(image.Repository) || strings.ContainsAny(image.Repository, "\t\r\n") {
		return fmt.Errorf("invalid OCI repository %q: %w", image.Repository, core.ErrInvalidArgument)
	}
	if strings.TrimSpace(image.Tag) != image.Tag || hasControl(image.Tag) || strings.ContainsAny(image.Tag, "\t\r\n") {
		return fmt.Errorf("invalid OCI tag %q: %w", image.Tag, core.ErrInvalidArgument)
	}
	if image.Digest != "" && !digestPattern.MatchString(image.Digest) {
		return fmt.Errorf("invalid OCI digest %q: %w", image.Digest, core.ErrIncompatibleState)
	}
	return nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
