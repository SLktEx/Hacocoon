package run

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeRunStore struct {
	runs       map[string]core.EphemeralRun
	putErr     error
	deleteErr  error
	listErr    error
	operations []string
}

func newFakeRunStore(runs ...core.EphemeralRun) *fakeRunStore {
	store := &fakeRunStore{runs: map[string]core.EphemeralRun{}}
	for _, run := range runs {
		store.runs[run.EnvironmentID] = run
	}
	return store
}

func (s *fakeRunStore) ListEphemeralRuns(context.Context) ([]core.EphemeralRun, error) {
	s.operations = append(s.operations, "list")
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]core.EphemeralRun, 0, len(s.runs))
	for _, run := range s.runs {
		result = append(result, run)
	}
	return result, nil
}

func (s *fakeRunStore) PutEphemeralRun(_ context.Context, run core.EphemeralRun) error {
	s.operations = append(s.operations, "put:"+run.EnvironmentID+":"+string(run.State))
	if s.putErr != nil {
		return s.putErr
	}
	if s.runs == nil {
		s.runs = map[string]core.EphemeralRun{}
	}
	s.runs[run.EnvironmentID] = run
	return nil
}

func (s *fakeRunStore) DeleteEphemeralRun(_ context.Context, environmentID string) error {
	s.operations = append(s.operations, "delete:"+environmentID)
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.runs, environmentID)
	return nil
}

type fakeOwnershipLock struct {
	released bool
	err      error
}

func (l *fakeOwnershipLock) Release() error {
	l.released = true
	return l.err
}

func recoveryService(env *fakeEnvironments, store *fakeRunStore) *Service {
	service := NewWithRecovery(env, store, "/ignored/run-locks")
	service.now = func() time.Time { return time.Date(2026, 8, 30, 4, 20, 0, 0, time.UTC) }
	return service
}

func TestReconcileSkipsEphemeralRunOwnedByLiveProcess(t *testing.T) {
	run := core.EphemeralRun{EnvironmentID: "run-live", State: core.EphemeralRunActive, CreatedAt: time.Now().UTC()}
	store := newFakeRunStore(run)
	env := &fakeEnvironments{}
	service := recoveryService(env, store)
	service.acquireOwnership = func(_, environmentID string, nonBlocking bool) (runOwnershipLock, bool, error) {
		if environmentID != run.EnvironmentID || !nonBlocking {
			t.Fatalf("ownership probe=%q nonBlocking=%t", environmentID, nonBlocking)
		}
		return nil, false, nil
	}

	if err := service.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(env.calls) != 0 {
		t.Fatalf("live run was touched: %v", env.calls)
	}
	if _, ok := store.runs[run.EnvironmentID]; !ok {
		t.Fatal("live run marker was removed")
	}
}

func TestReconcileDeletesOnlyMarkedRunAfterOwnershipIsFree(t *testing.T) {
	run := core.EphemeralRun{EnvironmentID: "ordinary-looking-name", State: core.EphemeralRunActive, CreatedAt: time.Now().UTC()}
	store := newFakeRunStore(run)
	env := &fakeEnvironments{}
	service := recoveryService(env, store)
	lock := &fakeOwnershipLock{}
	service.acquireOwnership = func(_, environmentID string, nonBlocking bool) (runOwnershipLock, bool, error) {
		if environmentID != run.EnvironmentID || !nonBlocking {
			t.Fatalf("ownership probe=%q nonBlocking=%t", environmentID, nonBlocking)
		}
		return lock, true, nil
	}

	if err := service.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(env.calls, []string{"delete:" + run.EnvironmentID}) {
		t.Fatalf("calls=%v", env.calls)
	}
	if _, ok := store.runs[run.EnvironmentID]; ok {
		t.Fatal("recovered marker was not removed")
	}
	if !lock.released {
		t.Fatal("ownership probe lock was not released")
	}
}

func TestReconcileNeverUsesRunNameAsDeletionAuthority(t *testing.T) {
	store := newFakeRunStore()
	env := &fakeEnvironments{}
	service := recoveryService(env, store)
	service.acquireOwnership = func(_, _ string, _ bool) (runOwnershipLock, bool, error) {
		t.Fatal("ownership must not be probed without a durable marker")
		return nil, false, nil
	}

	if err := service.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(env.calls) != 0 {
		t.Fatalf("unmarked run-* environment could be deleted: %v", env.calls)
	}
}

func TestReconcileKeepsMarkerWhenCleanupFails(t *testing.T) {
	cleanupErr := errors.New("runtime delete failed")
	run := core.EphemeralRun{EnvironmentID: "run-stale", State: core.EphemeralRunActive, CreatedAt: time.Now().UTC()}
	store := newFakeRunStore(run)
	env := &fakeEnvironments{deleteErr: cleanupErr}
	service := recoveryService(env, store)
	service.acquireOwnership = func(_, _ string, _ bool) (runOwnershipLock, bool, error) {
		return &fakeOwnershipLock{}, true, nil
	}

	err := service.Reconcile(context.Background())
	if !errors.Is(err, cleanupErr) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	got, ok := store.runs[run.EnvironmentID]
	if !ok || got.State != core.EphemeralRunCleanupRequired {
		t.Fatalf("marker=%#v present=%t", got, ok)
	}
}

func TestRunPersistsMarkerBeforeEnvironmentCreationAndRemovesItAfterCleanup(t *testing.T) {
	store := newFakeRunStore()
	env := &markerCheckingEnvironment{store: store}
	service := NewWithRecovery(env, store, "/ignored/run-locks")
	service.newName = func() (string, error) { return "run-marked", nil }
	service.now = func() time.Time { return time.Date(2026, 8, 30, 4, 21, 0, 0, time.UTC) }
	service.acquireOwnership = func(_, _ string, _ bool) (runOwnershipLock, bool, error) {
		return &fakeOwnershipLock{}, true, nil
	}

	result, err := service.Run(context.Background(), Spec{WorkspacePath: "/work/demo", Argv: []string{"true"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Environment != "run-marked" || !result.CleanedUp {
		t.Fatalf("result=%#v", result)
	}
	if !env.markerSeenDuringCreate {
		t.Fatal("environment creation started before durable ephemeral marker existed")
	}
	if len(store.runs) != 0 {
		t.Fatalf("completed marker remained: %#v", store.runs)
	}
}

func TestRunLeavesCleanupRequiredMarkerWhenDeleteFails(t *testing.T) {
	cleanupErr := errors.New("delete failed")
	store := newFakeRunStore()
	env := &fakeEnvironments{deleteErr: cleanupErr}
	service := NewWithRecovery(env, store, "/ignored/run-locks")
	service.newName = func() (string, error) { return "run-cleanup-required", nil }
	service.now = func() time.Time { return time.Date(2026, 8, 30, 4, 22, 0, 0, time.UTC) }
	service.acquireOwnership = func(_, _ string, _ bool) (runOwnershipLock, bool, error) {
		return &fakeOwnershipLock{}, true, nil
	}

	result, err := service.Run(context.Background(), Spec{WorkspacePath: "/work/demo", Argv: []string{"true"}})
	if !errors.Is(err, cleanupErr) || result.CleanedUp {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	marker, ok := store.runs["run-cleanup-required"]
	if !ok || marker.State != core.EphemeralRunCleanupRequired {
		t.Fatalf("marker=%#v present=%t", marker, ok)
	}
}

type markerCheckingEnvironment struct {
	store                  *fakeRunStore
	markerSeenDuringCreate bool
}

func (e *markerCheckingEnvironment) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	marker, ok := e.store.runs[spec.Name]
	e.markerSeenDuringCreate = ok && marker.State == core.EphemeralRunCreating
	return core.Environment{Name: spec.Name}, nil
}

func (*markerCheckingEnvironment) Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}

func (*markerCheckingEnvironment) Delete(context.Context, string) error { return nil }
