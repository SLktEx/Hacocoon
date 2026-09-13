package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"sync"
	"testing"
	"time"
)

type accessRuntimeFixture struct {
	fakeEnvironmentRuntime
	mu     sync.Mutex
	state  core.EnvironmentState
	starts int
}

func (r *accessRuntimeFixture) VerifyEnvironmentIdentity(context.Context, string, string) error {
	return nil
}
func (r *accessRuntimeFixture) InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return core.EnvironmentRuntimeStatus{State: r.state}, nil
}
func (r *accessRuntimeFixture) StartEnvironment(context.Context, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	r.starts++
	r.state = core.EnvironmentRunning
	return nil
}
func accessFixture() (*Service, *accessRuntimeFixture, *fakeEnvironmentStore, core.StreamTarget) {
	store := resumableStore()
	l := store.leases["resume"]
	l.InstanceID = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	l.Owner = "resume"
	l.AcquiredAt = time.Now().UTC()
	env := store.environments["resume"]
	env.CreatedAt = l.AcquiredAt
	store.environments["resume"] = env
	store.leases["resume"] = l
	runtime := &accessRuntimeFixture{state: core.EnvironmentStopped}
	return New(runtime, store), runtime, store, core.StreamTarget{Environment: "resume", Instance: l.InstanceID, Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"}
}
func TestConcurrentStreamResumeUsesOneCanonicalStart(t *testing.T) {
	s, r, _, target := accessFixture()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.WithClientAccess(context.Background(), "resume", &target, true, nil, func(core.Environment, string) error { return nil }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if r.starts != 1 {
		t.Fatalf("starts=%d", r.starts)
	}
}
func TestStreamRejectsStaleBindingsBeforeStart(t *testing.T) {
	for _, kind := range []string{"instance", "workspace", "mode", "deleted", "recovery", "revoked", "owner", "time", "snapshot"} {
		t.Run(kind, func(t *testing.T) {
			s, r, store, target := accessFixture()
			switch kind {
			case "instance":
				target.Instance = "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			case "workspace":
				target.Workspace = "other"
			case "mode":
				target.AccessMode = core.WorkspaceReadOnly
			case "deleted":
				delete(store.environments, "resume")
			case "owner", "time", "snapshot":
				l := store.leases["resume"]
				if kind == "owner" {
					l.Owner = "other"
				}
				if kind == "time" {
					l.AcquiredAt = l.AcquiredAt.Add(time.Second)
				}
				if kind == "snapshot" {
					l.SnapshotSource = "pending"
				}
				store.leases["resume"] = l
			case "recovery":
				l := store.leases["resume"]
				l.State = core.WorkspaceLeaseCleanupRequired
				store.leases["resume"] = l
			}
			authorize := func(core.Environment, string) error {
				if kind == "revoked" {
					return core.ErrPolicyDenied
				}
				return nil
			}
			err := s.WithClientAccess(context.Background(), "resume", &target, true, authorize, func(core.Environment, string) error { t.Fatal("opened rejected target"); return nil })
			if err == nil || r.starts != 0 {
				t.Fatalf("%v starts=%d", err, r.starts)
			}
			if kind == "revoked" && !errors.Is(err, core.ErrPolicyDenied) {
				t.Fatal(err)
			}
		})
	}
}
