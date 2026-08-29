package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	runtimeCaps, err := m.runtime.Probe(ctx)
	if err != nil {
		return StorageHandle{}, fmt.Errorf("probe runtime %s: %w", m.runtime.ID(), err)
	}
	if !runtimeCaps.Available {
		return StorageHandle{}, fmt.Errorf("%w: %s", ErrRuntimeUnavailable, m.runtime.ID())
	}

	storageCaps, err := m.storage.Probe(ctx)
	if err != nil {
		return StorageHandle{}, fmt.Errorf("probe storage %s: %w", m.storage.ID(), err)
	}
	if !storageCaps.Available {
		return StorageHandle{}, fmt.Errorf("%w: %s", ErrStorageUnavailable, m.storage.ID())
	}
	handle, err := m.storage.Ensure(ctx, StorageSpec{ID: defaultStorageID, SizeBytes: defaultStorageBytes})
	if err != nil {
		return StorageHandle{}, err
	}
	if preparer, ok := m.runtime.(RuntimePreparer); ok {
		if err := preparer.Prepare(ctx, RuntimePrepareSpec{StorageAttachment: cloneMap(handle.Attachment)}); err != nil {
			return StorageHandle{}, fmt.Errorf("prepare runtime %s: %w", m.runtime.ID(), err)
		}
	}
	return handle, nil
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
		cleanupErr := m.runtime.Delete(ctx, created.Ref)
		if cleanupErr != nil {
			return Session{}, errors.Join(
				fmt.Errorf("persist session: %w", err),
				fmt.Errorf("cleanup runtime %s: %w", created.Ref, cleanupErr),
				ErrRecoveryRequired,
			)
		}
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

// ShrinkStorage performs the session-aware safety check that cannot live inside a
// concrete storage module. The storage module remains responsible for filesystem
// and backing-image ordering; Core only verifies that Sessions using this storage
// are actually stopped before the destructive operation begins.
func (m *Manager) ShrinkStorage(ctx context.Context, handle StorageHandle, plan ShrinkPlan) error {
	resizable, ok := m.storage.(ResizableStorage)
	if !ok {
		return ErrUnsupported
	}
	if err := m.ensureStorageQuiesced(ctx, handle.ID); err != nil {
		return err
	}
	return resizable.Shrink(ctx, handle, plan)
}

func (m *Manager) ensureStorageQuiesced(ctx context.Context, storageRef string) error {
	sessions, err := m.store.List(ctx)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if session.StorageRef != storageRef {
			continue
		}
		state, inspectErr := m.runtime.Inspect(ctx, session.RuntimeRef)
		if inspectErr != nil {
			return fmt.Errorf("inspect session %s before storage shrink: %w", session.ID, inspectErr)
		}
		if state.Observed != ObservedStopped {
			return fmt.Errorf("%w: session %s is %s", ErrStorageBusy, session.ID, state.Observed)
		}
	}
	return nil
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
