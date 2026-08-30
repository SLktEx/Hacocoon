package seedbuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const stateVersion = 1

type ToolingManifest struct {
	Parent          core.BaseRef      `json:"parent"`
	ToolingRevision core.BaseRevision `json:"tooling_revision"`
	ToolingAlias    string            `json:"tooling_alias,omitempty"`
	BuiltAt         time.Time         `json:"built_at"`
}

type Manifest struct {
	Parent          core.BaseRef      `json:"parent"`
	ToolingRevision core.BaseRevision `json:"tooling_revision"`
	SeedRevision    core.BaseRevision `json:"seed_revision"`
	SeedAlias       string            `json:"seed_alias,omitempty"`
	Images          []ImageIdentity   `json:"images"`
	BuiltAt         time.Time         `json:"built_at"`
}

type fileState struct {
	Version int                        `json:"version"`
	Tooling map[string]ToolingManifest `json:"tooling"`
	Current map[string]Manifest        `json:"current"`
}

type Store struct {
	path    string
	mu      sync.Mutex
	buildMu sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

// CurrentSeed implements the Incus BaseProvider Seed resolver. A Seed is used
// only when its recorded parent exactly matches the currently resolved Base.
func (s *Store) CurrentSeed(ctx context.Context, parent core.BaseRef) (core.BaseRevision, bool, error) {
	manifest, ok, err := s.CurrentManifest(ctx, parent)
	if err != nil || !ok {
		return "", ok, err
	}
	return manifest.SeedRevision, true, nil
}

func (s *Store) CurrentManifest(_ context.Context, parent core.BaseRef) (Manifest, bool, error) {
	if s == nil || !validParent(parent) {
		return Manifest{}, false, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.path + ".lock")
	if err != nil {
		return Manifest{}, false, err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return Manifest{}, false, err
	}
	manifest, ok := state.Current[string(parent.Name)]
	if !ok || manifest.Parent != parent {
		return Manifest{}, false, nil
	}
	return manifest, true, nil
}

func (s *Store) ToolingManifest(_ context.Context, parent core.BaseRef) (ToolingManifest, bool, error) {
	if s == nil || !validParent(parent) {
		return ToolingManifest{}, false, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.path + ".lock")
	if err != nil {
		return ToolingManifest{}, false, err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return ToolingManifest{}, false, err
	}
	manifest, ok := state.Tooling[string(parent.Name)]
	if !ok || manifest.Parent != parent {
		return ToolingManifest{}, false, nil
	}
	return manifest, true, nil
}

func (s *Store) PutTooling(_ context.Context, manifest ToolingManifest) error {
	if s == nil || !validParent(manifest.Parent) || manifest.BuiltAt.IsZero() {
		return core.ErrInvalidArgument
	}
	if err := validateRevision(manifest.ToolingRevision); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return err
	}
	state.Tooling[string(manifest.Parent.Name)] = manifest
	return s.write(state)
}

func (s *Store) PutCurrent(_ context.Context, manifest Manifest) error {
	if s == nil || !validParent(manifest.Parent) || manifest.BuiltAt.IsZero() {
		return core.ErrInvalidArgument
	}
	if err := validateRevision(manifest.ToolingRevision); err != nil {
		return err
	}
	if err := validateRevision(manifest.SeedRevision); err != nil {
		return err
	}
	for _, image := range manifest.Images {
		if err := validateImageIdentity(image); err != nil {
			return err
		}
	}
	sort.Slice(manifest.Images, func(i, j int) bool {
		if manifest.Images[i].Reference != manifest.Images[j].Reference {
			return manifest.Images[i].Reference < manifest.Images[j].Reference
		}
		return manifest.Images[i].Digest < manifest.Images[j].Digest
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return err
	}
	state.Current[string(manifest.Parent.Name)] = manifest
	return s.write(state)
}

func (s *Store) LockBuild() (func(), error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, core.ErrInvalidArgument
	}
	s.buildMu.Lock()
	unlockFile, err := lockFile(s.path + ".build.lock")
	if err != nil {
		s.buildMu.Unlock()
		return nil, err
	}
	return func() {
		unlockFile()
		s.buildMu.Unlock()
	}, nil
}

func (s *Store) read() (fileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{Version: stateVersion, Tooling: map[string]ToolingManifest{}, Current: map[string]Manifest{}}, nil
	}
	if err != nil {
		return fileState{}, fmt.Errorf("read Seed state: %w", err)
	}
	var state fileState
	if err := json.Unmarshal(contents, &state); err != nil {
		return fileState{}, fmt.Errorf("decode Seed state: %w", err)
	}
	if state.Version != stateVersion {
		return fileState{}, fmt.Errorf("Seed state version %d is unsupported: %w", state.Version, core.ErrIncompatibleState)
	}
	if state.Tooling == nil {
		state.Tooling = map[string]ToolingManifest{}
	}
	if state.Current == nil {
		state.Current = map[string]Manifest{}
	}
	for name, manifest := range state.Tooling {
		if string(manifest.Parent.Name) != name || !validParent(manifest.Parent) || manifest.BuiltAt.IsZero() {
			return fileState{}, fmt.Errorf("invalid tooling Base state for %q: %w", name, core.ErrIncompatibleState)
		}
		if err := validateRevision(manifest.ToolingRevision); err != nil {
			return fileState{}, err
		}
	}
	for name, manifest := range state.Current {
		if string(manifest.Parent.Name) != name || !validParent(manifest.Parent) || manifest.BuiltAt.IsZero() {
			return fileState{}, fmt.Errorf("invalid current Seed state for %q: %w", name, core.ErrIncompatibleState)
		}
		if err := validateRevision(manifest.ToolingRevision); err != nil {
			return fileState{}, err
		}
		if err := validateRevision(manifest.SeedRevision); err != nil {
			return fileState{}, err
		}
		for _, image := range manifest.Images {
			if err := validateImageIdentity(image); err != nil {
				return fileState{}, err
			}
		}
	}
	return state, nil
}

func (s *Store) write(state fileState) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Seed state directory: %w", err)
	}
	state.Version = stateVersion
	if state.Tooling == nil {
		state.Tooling = map[string]ToolingManifest{}
	}
	if state.Current == nil {
		state.Current = map[string]Manifest{}
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Seed state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".seeds-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Seed state: %w", err)
	}
	name := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(name)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary Seed state: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temporary Seed state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary Seed state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("close temporary Seed state: %w", err)
	}
	if err := os.Rename(name, s.path); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("commit Seed state: %w", err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func validParent(parent core.BaseRef) bool {
	if strings.TrimSpace(string(parent.Name)) == "" {
		return false
	}
	return validateRevision(parent.Revision) == nil
}
