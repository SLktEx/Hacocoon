package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type legacyCleanupStore struct {
	*fakeEnvironmentStore
	op    core.SnapshotRestore
	fail  string
	gets  int
	trace []string
}

func (s *legacyCleanupStore) GetSnapshotRestore(context.Context, string) (core.SnapshotRestore, error) {
	s.gets++
	if s.fail == "get" || s.fail == "recheck" && s.gets == 2 {
		return core.SnapshotRestore{}, core.ErrRuntimeUnavailable
	}
	op := s.op
	if s.fail == "stale" && s.gets == 2 {
		op.Current.InstanceID = "replacement"
	}
	return op, nil
}
func (s *legacyCleanupStore) BeginRestoreCleanup(context.Context, string) error {
	s.trace = append(s.trace, "begin")
	if s.fail == "begin" {
		return core.ErrStorageBusy
	}
	return nil
}
func (s *legacyCleanupStore) RecordRestoreComponent(_ context.Context, _ string, c core.SnapshotComponent, next string) error {
	s.trace = append(s.trace, "record:"+c.Role+":"+next)
	if s.fail == "record" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (s *legacyCleanupStore) FinalizeRestoreCleanup(context.Context, string) error {
	s.trace = append(s.trace, "finalize")
	if s.fail == "finalize" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}

type legacyCleanupRuntime struct {
	fakeEnvironmentRuntime
	fail    bool
	deleted []core.SnapshotComponent
}

func (r *legacyCleanupRuntime) DeleteRestoreComponent(_ context.Context, c core.SnapshotComponent) error {
	r.deleted = append(r.deleted, c)
	if r.fail && c.Role == "rootfs" {
		return core.ErrStorageBusy
	}
	return nil
}

func TestLegacyCleanupNeverDeletesCurrentEnvironmentOrReleasesAmbiguousOwnership(t *testing.T) {
	for _, failure := range []string{"", "get", "recheck", "stale", "begin", "delete", "record", "finalize"} {
		t.Run(failure, func(t *testing.T) {
			env := core.Environment{Name: "work", Workspace: core.Workspace{ID: "workspace:managed:owned", Path: "managed:work"}, RuntimeRef: "existing"}
			s := &legacyCleanupStore{fakeEnvironmentStore: newFakeEnvironmentStore(), fail: failure, op: core.SnapshotRestore{ID: "old-stage", Current: core.SnapshotSource{Environment: env, InstanceID: "old-owner"}, Saved: core.Snapshot{ID: "saved"}, Components: []core.SnapshotComponent{{Role: "rootfs", NativeRef: "staged-root", Owner: "root-owner", State: "created"}, {Role: "workspace:main", NativeRef: "staged-work", Owner: "work-owner", State: "created"}, {Role: "oci", NativeRef: "absent", State: "absent"}}}}
			s.environments[env.Name] = env
			r := &legacyCleanupRuntime{fail: failure == "delete"}
			err := New(r, s).CleanupLegacySnapshotRestore(context.Background(), s.op.ID)
			if (err == nil) != (failure == "") {
				t.Fatal(failure, err)
			}
			if (failure == "delete" || failure == "record") && !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("ambiguous cleanup not retained", err)
			}
			if failure == "stale" && !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.environments[env.Name], env) || len(r.deleteRefs) != 0 {
				t.Fatal("current Environment changed")
			}
			if failure == "" && !reflect.DeepEqual(s.trace, []string{"begin", "record:rootfs:absent", "record:workspace:main:absent", "finalize"}) {
				t.Fatal(s.trace)
			}
			for _, c := range r.deleted {
				if c.State == "absent" || c.Owner == "" || c.NativeRef == env.RuntimeRef {
					t.Fatal("cleanup targeted wrong resource", c)
				}
			}
			if failure == "delete" && len(r.deleted) != 2 {
				t.Fatal("failure skipped independent cleanup", r.deleted)
			}
		})
	}
	if err := New(&fakeEnvironmentRuntime{}, newFakeEnvironmentStore()).CleanupLegacySnapshotRestore(context.Background(), "old"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if err := New(&legacyCleanupRuntime{}, newFakeEnvironmentStore()).CleanupLegacySnapshotRestore(context.Background(), "old"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}

type commitSourceRuntime struct {
	*quiesceRuntime
	calls   int
	failure error
}

func (r *commitSourceRuntime) CommitImage(_ context.Context, e core.Environment, l core.WorkspaceLease, n core.BaseName) (core.BaseInfo, error) {
	r.calls++
	if e.RuntimeRef != l.RuntimeRef || e.Workspace.ID != l.WorkspaceID {
		return core.BaseInfo{}, core.ErrCapabilityStale
	}
	return core.BaseInfo{Name: n, Revision: "rootfs-only"}, r.failure
}
func TestCommitKeepsRunningOrStoppedSourceUnchanged(t *testing.T) {
	for _, running := range []bool{false, true} {
		for _, failed := range []bool{false, true} {
			_, store, backend := captureFixture(t)
			r := &commitSourceRuntime{quiesceRuntime: &quiesceRuntime{captureRuntime: backend, running: running}}
			if failed {
				r.failure = core.ErrRuntimeUnavailable
			}
			before, err := store.GetEnvironment(context.Background(), "resume")
			if err != nil {
				t.Fatal(err)
			}
			image, err := New(r, store).Commit(context.Background(), "resume", "saved-image")
			if !errors.Is(err, r.failure) || r.calls != 1 || image.Name != "saved-image" {
				t.Fatal(image, err, r.calls)
			}
			after, err := store.GetEnvironment(context.Background(), "resume")
			if err != nil || !reflect.DeepEqual(before, after) || r.running != running {
				t.Fatal("commit modified source", err)
			}
			for _, event := range r.events {
				if event == "stop" || event == "start" {
					t.Fatal("commit changed runtime state", r.events)
				}
			}
		}
	}
}
