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
	Environments map[string]core.Environment `json:"environments"`
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

func (s *EnvironmentJSONStore) readEnvironments() (environmentFileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return environmentFileState{Environments: map[string]core.Environment{}}, nil
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
