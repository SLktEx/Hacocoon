package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type startingRuntime struct {
	fakeEnvironmentRuntime
	start func(context.Context, string) error
}

func (r *startingRuntime) StartEnvironment(ctx context.Context, ref string) error {
	return r.start(ctx, ref)
}
func resumableStore() *fakeEnvironmentStore {
	s := newFakeEnvironmentStore()
	s.environments["resume"] = core.Environment{Name: "resume", RuntimeRef: "provider:exact", Workspace: core.Workspace{ID: "work", Path: "managed:work"}, AccessMode: core.WorkspaceReadWrite}
	s.leases["resume"] = core.WorkspaceLease{EnvironmentID: "resume", RuntimeRef: "provider:exact", WorkspaceID: "work", SourcePath: "managed:work", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseActive}
	return s
}
func TestStartRetainsOwnershipOnSuccessAndUncertainFailure(t *testing.T) {
	for _, failure := range []error{nil, errors.New("uncertain start")} {
		s := resumableStore()
		before := s.environments["resume"]
		lease := s.leases["resume"]
		r := &startingRuntime{start: func(_ context.Context, ref string) error {
			if ref != before.RuntimeRef {
				t.Fatalf("wrong runtime %q", ref)
			}
			return failure
		}}
		if err := New(r, s).Start(context.Background(), "resume"); !errors.Is(err, failure) {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s.environments["resume"], before) || s.leases["resume"] != lease || len(r.deleteRefs) != 0 {
			t.Fatal("resume changed ownership")
		}
	}
}
func TestStartRejectsIncompleteOrMismatchedOwnership(t *testing.T) {
	for _, mutate := range []func(*core.WorkspaceLease){
		func(l *core.WorkspaceLease) { l.State = core.WorkspaceLeaseCleanupRequired },
		func(l *core.WorkspaceLease) { l.State = core.WorkspaceLeaseAcquiring },
		func(l *core.WorkspaceLease) { l.RuntimeRef = "other" },
		func(l *core.WorkspaceLease) { l.WorkspaceID = "other" },
		func(l *core.WorkspaceLease) { l.SourcePath = "other" },
		func(l *core.WorkspaceLease) { l.EnvironmentID = "other" },
		func(l *core.WorkspaceLease) { l.AccessMode = core.WorkspaceReadOnly },
	} {
		s := resumableStore()
		l := s.leases["resume"]
		mutate(&l)
		s.leases["resume"] = l
		r := &startingRuntime{start: func(context.Context, string) error { t.Fatal("unowned runtime started"); return nil }}
		if err := New(r, s).Start(context.Background(), "resume"); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal(err)
		}
	}
}
func TestDeleteCannotOvertakeStartAcrossServices(t *testing.T) {
	s := resumableStore()
	entered := make(chan struct{})
	release := make(chan struct{})
	r := &startingRuntime{start: func(context.Context, string) error { close(entered); <-release; return nil }}
	done := make(chan error, 1)
	go func() { done <- New(r, s).Start(context.Background(), "resume") }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	err := New(r, s).Delete(ctx, "resume")
	cancel()
	close(release)
	if startErr := <-done; startErr != nil {
		t.Fatal(startErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("delete overtook start: %v", err)
	}
	if len(r.deleteRefs) != 0 {
		t.Fatal("deleted live start target")
	}
	if err := New(r, s).Delete(context.Background(), "resume"); err != nil {
		t.Fatal(err)
	}
}
