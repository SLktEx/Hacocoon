package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const defaultStorageID = "local-default"

const defaultStorageBytes int64 = 128 << 30

type Manager struct {
	runtime Runtime
	storage Storage
	store   SessionStore
	now     func() time.Time
	newID   func() (SessionID, error)
}

func NewManager(runtime Runtime, storage Storage, store SessionStore) *Manager {
	return &Manager{
		runtime: runtime,
		storage: storage,
		store:   store,
		now:     time.Now,
		newID:   randomSessionID,
	}
}

func (m *Manager) Init(ctx context.Context) (StorageHandle, error) {
	caps, err := m.storage.Probe(ctx)
	if err != nil {
		return StorageHandle{}, fmt.Errorf("probe storage %s: %w", m.storage.ID(), err)
	}
	if !caps.Available {
		return StorageHandle{}, fmt.Errorf("%w: %s", ErrStorageUnavailable, m.storage.ID())
	}
	return m.storage.Ensure(ctx, StorageSpec{ID: defaultStorageID, SizeBytes: defaultStorageBytes})
}

func (m *Manager) Create(ctx context.Context, spec SessionSpec) (Session, error) {
	storage, err := m.Init(ctx)
	if err != nil {
		return Session{}, err
	}

	id, err := m.newID()
	if err != nil {
		return Session{}, fmt.Errorf("allocate session id: %w", err)
	}
	name := spec.Name
	if name == "" {
		name = "session-" + string(id)[:8]
	}

	created, err := m.runtime.Create(ctx, RuntimeSessionSpec{
		ID:                id,
		Name:              name,
		StorageAttachment: cloneMap(storage.Attachment),
	})
	if err != nil {
		return Session{}, fmt.Errorf("create runtime session: %w", err)
	}

	now := m.now().UTC()
	session := Session{
		ID:            id,
		Name:          name,
		RuntimeModule: m.runtime.ID(),
		RuntimeRef:    created.Ref,
		StorageModule: m.storage.ID(),
		StorageRef:    storage.ID,
		DesiredState:  DesiredRunning,
		ObservedState: ObservedRunning,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := m.store.Put(ctx, session); err != nil {
		_ = m.runtime.Delete(ctx, created.Ref)
		return Session{}, fmt.Errorf("persist session: %w", err)
	}
	return session, nil
}

func (m *Manager) List(ctx context.Context) ([]Session, error) {
	return m.store.List(ctx)
}

func (m *Manager) Get(ctx context.Context, id SessionID) (Session, error) {
	return m.store.Get(ctx, id)
}

func (m *Manager) Start(ctx context.Context, id SessionID) error {
	return m.transition(ctx, id, DesiredRunning, ObservedRunning, m.runtime.Start)
}

func (m *Manager) Stop(ctx context.Context, id SessionID) error {
	return m.transition(ctx, id, DesiredStopped, ObservedStopped, m.runtime.Stop)
}

func (m *Manager) Remove(ctx context.Context, id SessionID) error {
	session, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := m.runtime.Delete(ctx, session.RuntimeRef); err != nil {
		return fmt.Errorf("delete runtime %s: %w", session.RuntimeRef, err)
	}
	return m.store.Delete(ctx, id)
}

func (m *Manager) Exec(ctx context.Context, id SessionID, req ExecRequest) (ExecResult, error) {
	session, err := m.store.Get(ctx, id)
	if err != nil {
		return ExecResult{}, err
	}
	return m.runtime.Exec(ctx, session.RuntimeRef, req)
}

func (m *Manager) Reconcile(ctx context.Context) error {
	sessions, err := m.store.List(ctx)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		state, inspectErr := m.runtime.Inspect(ctx, session.RuntimeRef)
		observed := state.Observed
		if inspectErr != nil {
			observed = ObservedError
		}
		if observed == session.ObservedState {
			continue
		}
		session.ObservedState = observed
		session.UpdatedAt = m.now().UTC()
		if err := m.store.Put(ctx, session); err != nil {
			return fmt.Errorf("persist reconciled session %s: %w", session.ID, err)
		}
	}
	return nil
}

func (m *Manager) transition(
	ctx context.Context,
	id SessionID,
	desired DesiredState,
	observed ObservedState,
	action func(context.Context, string) error,
) error {
	session, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := action(ctx, session.RuntimeRef); err != nil {
		return err
	}
	session.DesiredState = desired
	session.ObservedState = observed
	session.UpdatedAt = m.now().UTC()
	return m.store.Put(ctx, session)
}

func randomSessionID() (SessionID, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return SessionID(hex.EncodeToString(b[:])), nil
}

func cloneMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
