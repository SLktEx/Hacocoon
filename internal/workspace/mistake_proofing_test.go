package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// lifecycleOnlyStore intentionally implements only the high-level mutation
// surface accepted by Service. If low-level Environment/lease mutations are
// reintroduced into environmentStore, this compile-time assertion fails and
// forces the architecture change to be explicit.
type lifecycleOnlyStore struct{}

func (*lifecycleOnlyStore) GetEnvironment(context.Context, string) (core.Environment, error) {
	return core.Environment{}, core.ErrNotFound
}
func (*lifecycleOnlyStore) GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error) {
	return core.WorkspaceLease{}, core.ErrNotFound
}
func (*lifecycleOnlyStore) BeginEnvironmentCreate(context.Context, core.WorkspaceLease) error {
	return nil
}
func (*lifecycleOnlyStore) RecordEnvironmentRuntime(context.Context, core.WorkspaceLease) error {
	return nil
}
func (*lifecycleOnlyStore) CommitEnvironmentCreate(context.Context, core.Environment, core.WorkspaceLease) error {
	return nil
}
func (*lifecycleOnlyStore) MarkEnvironmentRecoveryRequired(context.Context, core.WorkspaceLease) error {
	return nil
}
func (*lifecycleOnlyStore) FinalizeEnvironmentDelete(context.Context, string) error { return nil }

var _ environmentStore = (*lifecycleOnlyStore)(nil)

type transitionRecordingStore struct {
	*fakeEnvironmentStore
	transitions []string
}

func newTransitionRecordingStore() *transitionRecordingStore {
	return &transitionRecordingStore{fakeEnvironmentStore: newFakeEnvironmentStore()}
}

func (s *transitionRecordingStore) BeginEnvironmentCreate(ctx context.Context, lease core.WorkspaceLease) error {
	s.transitions = append(s.transitions, "begin")
	return s.fakeEnvironmentStore.BeginEnvironmentCreate(ctx, lease)
}

func (s *transitionRecordingStore) RecordEnvironmentRuntime(ctx context.Context, lease core.WorkspaceLease) error {
	s.transitions = append(s.transitions, "record-runtime")
	return s.fakeEnvironmentStore.RecordEnvironmentRuntime(ctx, lease)
}

func (s *transitionRecordingStore) CommitEnvironmentCreate(ctx context.Context, environment core.Environment, lease core.WorkspaceLease) error {
	s.transitions = append(s.transitions, "commit-ready")
	return s.fakeEnvironmentStore.CommitEnvironmentCreate(ctx, environment, lease)
}

func (s *transitionRecordingStore) MarkEnvironmentRecoveryRequired(ctx context.Context, lease core.WorkspaceLease) error {
	s.transitions = append(s.transitions, "mark-recovery")
	return s.fakeEnvironmentStore.MarkEnvironmentRecoveryRequired(ctx, lease)
}

func (s *transitionRecordingStore) FinalizeEnvironmentDelete(ctx context.Context, environmentID string) error {
	s.transitions = append(s.transitions, "finalize-delete")
	return s.fakeEnvironmentStore.FinalizeEnvironmentDelete(ctx, environmentID)
}

func TestCreateRecordsProviderOwnershipBeforeReadyCommit(t *testing.T) {
	workspacePath := filepath.Clean(t.TempDir())
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
	store := newTransitionRecordingStore()
	service := New(runtime, store)
	service.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }

	if _, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspacePath}); err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", "record-runtime", "commit-ready"}
	if !reflect.DeepEqual(store.transitions, want) {
		t.Fatalf("lifecycle transitions = %#v, want %#v", store.transitions, want)
	}
}

func TestCreateFailsClosedWhenProviderCannotReturnRuntimeIdentity(t *testing.T) {
	workspacePath := filepath.Clean(t.TempDir())
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{}}
	store := newTransitionRecordingStore()

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspacePath})
	if !errors.Is(err, core.ErrIncompatibleState) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	lease, ok := store.leases["demo"]
	if !ok || lease.State != core.WorkspaceLeaseCleanupRequired {
		t.Fatalf("recovery reservation = %#v", lease)
	}
	want := []string{"begin", "mark-recovery"}
	if !reflect.DeepEqual(store.transitions, want) {
		t.Fatalf("lifecycle transitions = %#v, want %#v", store.transitions, want)
	}
}
