package agenthost

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

const bindingStateVersion = 1

type bindingRecord struct {
	EnvironmentName string                   `json:"environment"`
	WorkspacePath   string                   `json:"workspace_path"`
	AccessMode      core.WorkspaceAccessMode `json:"access_mode"`
}

type bindingStore interface {
	Get(context.Context, string) (bindingRecord, error)
	PutIfAbsent(context.Context, string, bindingRecord) error
	Delete(context.Context, string) error
}

type bindingFileState struct {
	Version  int                      `json:"version"`
	Bindings map[string]bindingRecord `json:"bindings"`
}

type JSONBindingStore struct {
	path string
	mu   sync.Mutex
}

func NewJSONBindingStore(path string) *JSONBindingStore {
	return &JSONBindingStore{path: path}
}

func (s *JSONBindingStore) Get(_ context.Context, sessionKey string) (bindingRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockBindingState(s.path)
	if err != nil {
		return bindingRecord{}, err
	}
	defer unlock()

	state, err := s.read()
	if err != nil {
		return bindingRecord{}, err
	}
	record, ok := state.Bindings[sessionKey]
	if !ok {
		return bindingRecord{}, fmt.Errorf("agent binding %q: %w", sessionKey, core.ErrNotFound)
	}
	return record, nil
}

func (s *JSONBindingStore) PutIfAbsent(_ context.Context, sessionKey string, record bindingRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockBindingState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	state, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := state.Bindings[sessionKey]; ok {
		return fmt.Errorf("agent binding %q: %w", sessionKey, core.ErrAlreadyExists)
	}
	state.Bindings[sessionKey] = record
	return s.write(state)
}

func (s *JSONBindingStore) Delete(_ context.Context, sessionKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockBindingState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	state, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := state.Bindings[sessionKey]; !ok {
		return nil
	}
	delete(state.Bindings, sessionKey)
	return s.write(state)
}

func (s *JSONBindingStore) read() (bindingFileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return bindingFileState{Version: bindingStateVersion, Bindings: map[string]bindingRecord{}}, nil
	}
	if err != nil {
		return bindingFileState{}, fmt.Errorf("read agent binding state: %w", err)
	}
	var state bindingFileState
	if err := json.Unmarshal(contents, &state); err != nil {
		return bindingFileState{}, fmt.Errorf("decode agent binding state: %w", err)
	}
	if state.Version != bindingStateVersion {
		return bindingFileState{}, fmt.Errorf("agent binding state version %d is unsupported (want %d): %w", state.Version, bindingStateVersion, core.ErrIncompatibleState)
	}
	if state.Bindings == nil {
		state.Bindings = map[string]bindingRecord{}
	}
	return state, nil
}

func (s *JSONBindingStore) write(state bindingFileState) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create agent binding state directory: %w", err)
	}
	state.Version = bindingStateVersion
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode agent binding state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".agent-bindings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary agent binding state: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary agent binding state: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temporary agent binding state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary agent binding state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary agent binding state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit agent binding state: %w", err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
