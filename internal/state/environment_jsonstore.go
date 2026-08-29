package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const environmentStateVersion = 2

type environmentFileState struct {
	Version      int                            `json:"version"`
	Environments map[string]core.Environment    `json:"environments"`
	Leases       map[string]core.WorkspaceLease `json:"workspace_leases,omitempty"`
}

type EnvironmentJSONStore struct {
	path string
	mu   sync.Mutex
}

func NewEnvironmentJSONStore(path string) *EnvironmentJSONStore {
	return &EnvironmentJSONStore{path: path}
}

func (s *EnvironmentJSONStore) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return core.Environment{}, err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return core.Environment{}, err
	}
	environment, ok := data.Environments[name]
	if !ok {
		return core.Environment{}, fmt.Errorf("environment %q: %w", name, core.ErrNotFound)
	}
	return environment, nil
}

func (s *EnvironmentJSONStore) PutEnvironment(_ context.Context, environment core.Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	data.Environments[environment.Name] = environment
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) DeleteEnvironment(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if _, ok := data.Environments[name]; !ok {
		return nil
	}
	delete(data.Environments, name)
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) ListWorkspaceLeases(_ context.Context) ([]core.WorkspaceLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return nil, err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return nil, err
	}
	leases := make([]core.WorkspaceLease, 0, len(data.Leases))
	for _, lease := range data.Leases {
		leases = append(leases, lease)
	}
	return leases, nil
}

func (s *EnvironmentJSONStore) GetWorkspaceLease(_ context.Context, environmentID string) (core.WorkspaceLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return core.WorkspaceLease{}, err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return core.WorkspaceLease{}, err
	}
	lease, ok := data.Leases[environmentID]
	if !ok {
		return core.WorkspaceLease{}, fmt.Errorf("workspace lease for environment %q: %w", environmentID, core.ErrNotFound)
	}
	return lease, nil
}

func (s *EnvironmentJSONStore) AcquireWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if _, ok := data.Environments[lease.EnvironmentID]; ok {
		return fmt.Errorf("environment %q: %w", lease.EnvironmentID, core.ErrAlreadyExists)
	}
	if _, ok := data.Leases[lease.EnvironmentID]; ok {
		return fmt.Errorf("workspace lease for environment %q already exists: %w", lease.EnvironmentID, core.ErrAlreadyExists)
	}
	for _, existing := range data.Leases {
		if existing.WorkspaceID != lease.WorkspaceID {
			continue
		}
		if existing.AccessMode == core.WorkspaceReadWrite || lease.AccessMode == core.WorkspaceReadWrite {
			return fmt.Errorf("workspace %s already leased by environment %q (%s): %w", lease.WorkspaceID, existing.EnvironmentID, existing.AccessMode, core.ErrWorkspaceBusy)
		}
	}
	if lease.State == "" {
		lease.State = core.WorkspaceLeaseAcquiring
	}
	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) PutWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if _, ok := data.Leases[lease.EnvironmentID]; !ok {
		return fmt.Errorf("workspace lease for environment %q: %w", lease.EnvironmentID, core.ErrNotFound)
	}
	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) DeleteWorkspaceLease(_ context.Context, environmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if _, ok := data.Leases[environmentID]; !ok {
		return nil
	}
	delete(data.Leases, environmentID)
	return s.writeEnvironments(data)
}

func newEnvironmentFileState() environmentFileState {
	return environmentFileState{
		Version:      environmentStateVersion,
		Environments: map[string]core.Environment{},
		Leases:       map[string]core.WorkspaceLease{},
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
	if err := normalizeEnvironmentState(&data); err != nil {
		return environmentFileState{}, err
	}
	return data, nil
}

func normalizeEnvironmentState(data *environmentFileState) error {
	if data.Version != 0 && data.Version != environmentStateVersion {
		return fmt.Errorf("environment state version %d is unsupported (want %d): %w", data.Version, environmentStateVersion, core.ErrIncompatibleState)
	}

	for name, environment := range data.Environments {
		if environment.Name == "" {
			environment.Name = name
		}
		if environment.Workspace.ID == "" || environment.Workspace.Path == "" || environment.AccessMode == "" || environment.RuntimeRef == "" {
			return fmt.Errorf("environment %q uses pre-v0.2 metadata; delete v0.1 environments before upgrading: %w", name, core.ErrIncompatibleState)
		}
		data.Environments[name] = environment
		if _, ok := data.Leases[name]; !ok {
			data.Leases[name] = core.WorkspaceLease{
				WorkspaceID:   environment.Workspace.ID,
				SourcePath:    environment.Workspace.Path,
				EnvironmentID: name,
				AccessMode:    environment.AccessMode,
				Owner:         name,
				RuntimeRef:    environment.RuntimeRef,
				State:         core.WorkspaceLeaseActive,
				AcquiredAt:    environment.CreatedAt,
			}
		}
	}

	for environmentID, lease := range data.Leases {
		if lease.EnvironmentID == "" {
			lease.EnvironmentID = environmentID
		}
		if lease.State == "" {
			lease.State = core.WorkspaceLeaseActive
		}
		if lease.RuntimeRef == "" {
			if environment, ok := data.Environments[environmentID]; ok {
				lease.RuntimeRef = environment.RuntimeRef
			}
		}
		data.Leases[environmentID] = lease
	}
	data.Version = environmentStateVersion
	return nil
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
