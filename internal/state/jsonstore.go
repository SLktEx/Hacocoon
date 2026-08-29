package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fileState struct {
	Sessions map[core.SessionID]core.Session `json:"sessions"`
}

type JSONStore struct {
	path string
	mu   sync.Mutex
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func (s *JSONStore) List(context.Context) ([]core.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]core.Session, 0, len(data.Sessions))
	for _, session := range data.Sessions {
		out = append(out, session)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *JSONStore) Get(_ context.Context, id core.SessionID) (core.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil {
		return core.Session{}, err
	}
	session, ok := data.Sessions[id]
	if !ok {
		return core.Session{}, fmt.Errorf("session %s: %w", id, core.ErrNotFound)
	}
	return session, nil
}

func (s *JSONStore) Put(_ context.Context, session core.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil {
		return err
	}
	data.Sessions[session.ID] = session
	return s.write(data)
}

func (s *JSONStore) Delete(_ context.Context, id core.SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil {
		return err
	}
	delete(data.Sessions, id)
	return s.write(data)
}

func (s *JSONStore) read() (fileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{Sessions: map[core.SessionID]core.Session{}}, nil
	}
	if err != nil {
		return fileState{}, fmt.Errorf("read state: %w", err)
	}

	var data fileState
	if err := json.Unmarshal(contents, &data); err != nil {
		return fileState{}, fmt.Errorf("decode state: %w", err)
	}
	if data.Sessions == nil {
		data.Sessions = map[core.SessionID]core.Session{}
	}
	return data, nil
}

func (s *JSONStore) write(data fileState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("write temporary state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("commit state: %w", err)
	}
	return nil
}
