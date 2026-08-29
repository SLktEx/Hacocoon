package core

import (
	"context"
	"errors"
	"testing"
)

type fakeRuntime struct {
	created   bool
	deleted   bool
	state     ObservedState
	createErr error
}

func (*fakeRuntime) ID() string { return "runtime.fake" }
func (*fakeRuntime) Probe(context.Context) (RuntimeCapabilities, error) {
	return RuntimeCapabilities{Available: true}, nil
}
func (f *fakeRuntime) Create(context.Context, RuntimeSessionSpec) (RuntimeSession, error) {
	if f.createErr != nil {
		return RuntimeSession{}, f.createErr
	}
	f.created = true
	f.state = ObservedRunning
	return RuntimeSession{Ref: "runtime-ref"}, nil
}
func (f *fakeRuntime) Start(context.Context, string) error  { f.state = ObservedRunning; return nil }
func (f *fakeRuntime) Stop(context.Context, string) error   { f.state = ObservedStopped; return nil }
func (f *fakeRuntime) Delete(context.Context, string) error { f.deleted = true; return nil }
func (*fakeRuntime) Exec(context.Context, string, ExecRequest) (ExecResult, error) {
	return ExecResult{ExitCode: 0, Stdout: "ok"}, nil
}
func (f *fakeRuntime) Inspect(context.Context, string) (RuntimeState, error) {
	return RuntimeState{Observed: f.state}, nil
}

type fakeStorage struct {
	ensureErr    error
	shrinkCalled bool
}

func (*fakeStorage) ID() string { return "storage.fake" }
func (*fakeStorage) Probe(context.Context) (StorageCapabilities, error) {
	return StorageCapabilities{Available: true}, nil
}
func (f *fakeStorage) Ensure(context.Context, StorageSpec) (StorageHandle, error) {
	if f.ensureErr != nil {
		return StorageHandle{}, f.ensureErr
	}
	return StorageHandle{ID: "storage-ref", Attachment: map[string]string{"opaque": "value"}}, nil
}
func (*fakeStorage) Inspect(context.Context, StorageHandle) (StorageState, error) {
	return StorageState{Healthy: true}, nil
}
func (*fakeStorage) Delete(context.Context, StorageHandle) error      { return nil }
func (*fakeStorage) Grow(context.Context, StorageHandle, int64) error { return nil }
func (*fakeStorage) PlanShrink(_ context.Context, handle StorageHandle, target int64) (ShrinkPlan, error) {
	return ShrinkPlan{HandleID: handle.ID, CurrentBytes: 100 << 30, TargetBytes: target, Feasible: true}, nil
}
func (f *fakeStorage) Shrink(context.Context, StorageHandle, ShrinkPlan) error {
	f.shrinkCalled = true
	return nil
}
func (*fakeStorage) Compact(context.Context, StorageHandle) error { return nil }

type memStore struct {
	sessions map[SessionID]Session
	putErr   error
}

func newMemStore() *memStore { return &memStore{sessions: map[SessionID]Session{}} }
func (m *memStore) List(context.Context) ([]Session, error) {
	out := make([]Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out, nil
}
func (m *memStore) Get(_ context.Context, id SessionID) (Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return s, nil
}
func (m *memStore) Put(_ context.Context, s Session) error {
	if m.putErr != nil {
		return m.putErr
	}
	m.sessions[s.ID] = s
	return nil
}
func (m *memStore) Delete(_ context.Context, id SessionID) error { delete(m.sessions, id); return nil }

func TestCreateDoesNotNeedRepository(t *testing.T) {
	runtime := &fakeRuntime{}
	store := newMemStore()
	manager := NewManager(runtime, &fakeStorage{}, store)
	manager.newID = func() (SessionID, error) { return "0123456789abcdef", nil }

	session, err := manager.Create(context.Background(), SessionSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if !runtime.created {
		t.Fatal("runtime was not created")
	}
	if session.RuntimeModule != "runtime.fake" || session.StorageModule != "storage.fake" {
		t.Fatalf("unexpected module ids: %+v", session)
	}
	if session.Name != "session-01234567" {
		t.Fatalf("unexpected generated name %q", session.Name)
	}
}

func TestStorageFailurePreventsRuntimeCreate(t *testing.T) {
	runtime := &fakeRuntime{}
	manager := NewManager(runtime, &fakeStorage{ensureErr: errors.New("boom")}, newMemStore())
	_, err := manager.Create(context.Background(), SessionSpec{Name: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if runtime.created {
		t.Fatal("runtime must not be created after storage failure")
	}
}

func TestPersistFailureCleansRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	store := newMemStore()
	store.putErr = errors.New("disk full")
	manager := NewManager(runtime, &fakeStorage{}, store)
	manager.newID = func() (SessionID, error) { return "0123456789abcdef", nil }
	_, err := manager.Create(context.Background(), SessionSpec{Name: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !runtime.deleted {
		t.Fatal("created runtime must be cleaned up when state persistence fails")
	}
}

func TestReconcileUpdatesObservedState(t *testing.T) {
	runtime := &fakeRuntime{state: ObservedStopped}
	store := newMemStore()
	store.sessions["id"] = Session{ID: "id", RuntimeRef: "r", ObservedState: ObservedRunning, DesiredState: DesiredRunning}
	manager := NewManager(runtime, &fakeStorage{}, store)
	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(context.Background(), "id")
	if got.ObservedState != ObservedStopped {
		t.Fatalf("got %s", got.ObservedState)
	}
}

func TestShrinkStorageRefusesRunningSession(t *testing.T) {
	runtime := &fakeRuntime{state: ObservedRunning}
	storage := &fakeStorage{}
	store := newMemStore()
	store.sessions["id"] = Session{
		ID:            "id",
		RuntimeRef:    "runtime-ref",
		StorageRef:    "storage-ref",
		ObservedState: ObservedRunning,
	}
	manager := NewManager(runtime, storage, store)
	handle := StorageHandle{ID: "storage-ref"}
	plan, _ := storage.PlanShrink(context.Background(), handle, 80<<30)

	err := manager.ShrinkStorage(context.Background(), handle, plan)
	if !errors.Is(err, ErrStorageBusy) {
		t.Fatalf("expected ErrStorageBusy, got %v", err)
	}
	if storage.shrinkCalled {
		t.Fatal("storage shrink must not start while a session is running")
	}
}

func TestShrinkStorageAllowsStoppedSession(t *testing.T) {
	runtime := &fakeRuntime{state: ObservedStopped}
	storage := &fakeStorage{}
	store := newMemStore()
	store.sessions["id"] = Session{
		ID:            "id",
		RuntimeRef:    "runtime-ref",
		StorageRef:    "storage-ref",
		ObservedState: ObservedStopped,
	}
	manager := NewManager(runtime, storage, store)
	handle := StorageHandle{ID: "storage-ref"}
	plan, _ := storage.PlanShrink(context.Background(), handle, 80<<30)

	if err := manager.ShrinkStorage(context.Background(), handle, plan); err != nil {
		t.Fatal(err)
	}
	if !storage.shrinkCalled {
		t.Fatal("storage shrink was not called for a stopped session")
	}
}

func TestShrinkStorageIgnoresSessionsOnOtherStorage(t *testing.T) {
	runtime := &fakeRuntime{state: ObservedRunning}
	storage := &fakeStorage{}
	store := newMemStore()
	store.sessions["id"] = Session{
		ID:            "id",
		RuntimeRef:    "runtime-ref",
		StorageRef:    "other-storage",
		ObservedState: ObservedRunning,
	}
	manager := NewManager(runtime, storage, store)
	handle := StorageHandle{ID: "storage-ref"}
	plan, _ := storage.PlanShrink(context.Background(), handle, 80<<30)

	if err := manager.ShrinkStorage(context.Background(), handle, plan); err != nil {
		t.Fatal(err)
	}
	if !storage.shrinkCalled {
		t.Fatal("unrelated session should not block storage shrink")
	}
}
