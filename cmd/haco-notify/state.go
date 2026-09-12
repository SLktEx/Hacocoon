package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

const maxNotifyStateBytes = 128 * 1024

var errUnsafeNotifyState = errors.New("unsafe notification state")

type notifyStore struct {
	root   *os.Root
	name   string
	unlock func()
}

func stateDirectory(path string) (*notifyStore, error) {
	absolute, err := filepath.Abs(path)
	if err != nil || path == "" {
		return nil, interaction.ErrInvalidArgument
	}
	directory, name := filepath.Dir(absolute), filepath.Base(absolute)
	if name == "." || name == string(filepath.Separator) {
		return nil, interaction.ErrInvalidArgument
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	before, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errUnsafeNotifyState
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) || !ownedNotifyFile(after, true) {
		root.Close()
		return nil, errUnsafeNotifyState
	}
	return &notifyStore{root: root, name: name}, nil
}

func openNotifyStore(path string) (*notifyStore, error) {
	store, err := stateDirectory(path)
	if err != nil {
		return nil, err
	}
	store.unlock, err = lockNotifyState(store.root, store.name+".lock")
	if err != nil {
		store.close()
		return nil, err
	}
	return store, nil
}

func (s *notifyStore) close() {
	if s.unlock != nil {
		s.unlock()
	}
	s.root.Close()
}

func (s *notifyStore) load() (notifyState, error) {
	var state notifyState
	file, err := openNotifyFile(s.root, s.name, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, maxNotifyStateBytes+1))
	if err != nil {
		return state, err
	}
	if len(payload) > maxNotifyStateBytes {
		return state, errUnsafeNotifyState
	}
	if err := json.Unmarshal(payload, &state); err != nil || state.Offset < 0 {
		return notifyState{}, interaction.ErrInvalidArgument
	}
	if len(state.SeenEventIDs) > maxSeenEventIDs {
		state.SeenEventIDs = state.SeenEventIDs[len(state.SeenEventIDs)-maxSeenEventIDs:]
	}
	return state, nil
}

func (s *notifyStore) save(state notifyState) error {
	if state.Offset < 0 {
		return interaction.ErrInvalidArgument
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(payload) > maxNotifyStateBytes {
		return errUnsafeNotifyState
	}
	// Reject an existing linked or foreign state without reading or modifying it.
	existing, err := openNotifyFile(s.root, s.name, os.O_RDONLY, 0)
	if err == nil {
		existing.Close()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary := ".native-notify-" + rand.Text() + ".tmp"
	file, err := openNotifyFile(s.root, temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove(temporary)
	defer file.Close()
	if _, err = file.Write(payload); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = s.root.Rename(temporary, s.name); err != nil {
		return err
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func loadState(path string) (notifyState, error) {
	store, err := stateDirectory(path)
	if err != nil {
		return notifyState{}, err
	}
	defer store.close()
	return store.load()
}
func saveState(path string, state notifyState) error {
	store, err := openNotifyStore(path)
	if err != nil {
		return err
	}
	defer store.close()
	return store.save(state)
}
