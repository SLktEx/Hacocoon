package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const environmentStateVersion = 13 // 9 was an unpublished replacement prototype; reject it.
const previousEnvironmentStateVersion = 2

type environmentFileState struct {
	WorkspaceCopies     map[string]snapshotWorkspaceCopy   `json:"snapshot_workspace_copies,omitempty"`
	Restores            map[string]core.SnapshotRestore    `json:"restores,omitempty"`
	BaseAssets          map[string]core.BaseAsset          `json:"base_assets,omitempty"`
	Snapshots           map[string]core.Snapshot           `json:"snapshots,omitempty"`
	PersistentResources map[string]core.PersistentResource `json:"persistent_resources,omitempty"`
	Version             int                                `json:"version"`
	Environments        map[string]core.Environment        `json:"environments"`
	Leases              map[string]core.WorkspaceLease     `json:"workspace_leases,omitempty"`
	EphemeralRuns       map[string]core.EphemeralRun       `json:"ephemeral_runs,omitempty"`
}

type EnvironmentJSONStore struct {
	path string
	mu   sync.Mutex
}

func NewEnvironmentJSONStore(path string) *EnvironmentJSONStore {
	return &EnvironmentJSONStore{path: path}
}

func newEnvironmentFileState() environmentFileState {
	return environmentFileState{
		WorkspaceCopies:     map[string]snapshotWorkspaceCopy{},
		Restores:            map[string]core.SnapshotRestore{},
		BaseAssets:          map[string]core.BaseAsset{},
		Snapshots:           map[string]core.Snapshot{},
		PersistentResources: map[string]core.PersistentResource{},
		Version:             environmentStateVersion,
		Environments:        map[string]core.Environment{},
		Leases:              map[string]core.WorkspaceLease{},
		EphemeralRuns:       map[string]core.EphemeralRun{},
	}
}

func (s *EnvironmentJSONStore) readEnvironments() (environmentFileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return newEnvironmentFileState(), nil
	}
	if err != nil {
		return environmentFileState{}, fmt.Errorf("read environment state: %w", err)
	}

	var data environmentFileState
	if err := json.Unmarshal(contents, &data); err != nil {
		return environmentFileState{}, fmt.Errorf("decode environment state: %w", err)
	}
	if data.Environments == nil {
		data.Environments = map[string]core.Environment{}
	}
	if data.Leases == nil {
		data.Leases = map[string]core.WorkspaceLease{}
	}
	if data.EphemeralRuns == nil {
		data.EphemeralRuns = map[string]core.EphemeralRun{}
	}
	if data.WorkspaceCopies == nil {
		data.WorkspaceCopies = map[string]snapshotWorkspaceCopy{}
	}
	if data.Restores == nil {
		data.Restores = map[string]core.SnapshotRestore{}
	}
	if data.BaseAssets == nil {
		data.BaseAssets = map[string]core.BaseAsset{}
	}
	if data.Snapshots == nil {
		data.Snapshots = map[string]core.Snapshot{}
	}
	if data.PersistentResources == nil {
		data.PersistentResources = map[string]core.PersistentResource{}
	}
	if err := normalizeEnvironmentState(&data); err != nil {
		return environmentFileState{}, err
	}
	return data, nil
}

func (s *EnvironmentJSONStore) writeEnvironments(data environmentFileState) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create environment state directory: %w", err)
	}
	data.Version = environmentStateVersion
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode environment state: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".environments-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary environment state: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary environment state: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temporary environment state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary environment state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary environment state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit environment state: %w", err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
