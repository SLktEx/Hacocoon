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

type environmentFileState struct {
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

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if _, ok := data.Environments[name]; !ok {
		return fmt.Errorf("environment %q: %w", name, core.ErrNotFound)
	}
	delete(data.Environments, name)
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) ListWorkspaceLeases(_ context.Context) ([]core.WorkspaceLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *EnvironmentJSONStore) AcquireWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	for environmentID, existing := range data.Leases {
		if _, ok := data.Environments[environmentID]; !ok {
			delete(data.Leases, environmentID)
			continue
		}
		if existing.WorkspaceID != lease.WorkspaceID || existing.EnvironmentID == lease.EnvironmentID {
			continue
		}
		if existing.AccessMode == core.WorkspaceReadWrite || lease.AccessMode == core.WorkspaceReadWrite {
			return fmt.Errorf("workspace %s already leased by environment %q (%s): %w", lease.WorkspaceID, existing.EnvironmentID, existing.AccessMode, core.ErrWorkspaceBusy)
		}
	}
	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) PutWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) DeleteWorkspaceLease(_ context.Context, environmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if _, ok := data.Leases[environmentID]; !ok {
		return fmt.Errorf("workspace lease for environment %q: %w", environmentID, core.ErrNotFound)
	}
	delete(data.Leases, environmentID)
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) readEnvironments() (environmentFileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return environmentFileState{Environments: map[string]core.Environment{}, Leases: map[string]core.WorkspaceLease{}}, nil
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
	return data, nil
}

func (s *EnvironmentJSONStore) writeEnvironments(data environmentFileState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create environment state directory: %w", err)
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode environment state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("write temporary environment state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("commit environment state: %w", err)
	}
	return nil
}
