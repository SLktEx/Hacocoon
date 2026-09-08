package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
	"time"
)

type snapshotRuntime struct {
	fakeEnvironmentRuntime
	inspect func(context.Context, string) (core.EnvironmentRuntimeStatus, error)
}

func (r *snapshotRuntime) InspectEnvironment(ctx context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	return r.inspect(ctx, ref)
}

type snapshotStore struct {
	*fakeEnvironmentStore
	identity string
}

func (s *snapshotStore) EnvironmentInstance(context.Context, core.Environment) (string, error) {
	return s.identity, nil
}
func snapshotFixture() (*snapshotStore, *snapshotRuntime) {
	s := &snapshotStore{resumableStore(), "env-11111111111111111111111111111111"}
	l := s.leases["resume"]
	l.InstanceID = s.identity
	l.Owner = "snapshot-fixture"
	s.leases["resume"] = l
	r := &snapshotRuntime{inspect: func(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
		return core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}, nil
	}}
	return s, r
}
func TestSnapshotSourceRequiresCompleteStoppedAggregate(t *testing.T) {
	for _, mode := range []string{"valid", "valid-resource", "external", "running", "unknown", "drift", "recreated", "unowned", "resource-drift", "resource-invalid", "inspection-failure"} {
		t.Run(mode, func(t *testing.T) {
			s, r := snapshotFixture()
			l := s.leases["resume"]
			e := s.environments["resume"]
			switch mode {
			case "valid-resource":
				e.PersistentResource = core.PersistentResourceRef{ID: "oci:one", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
				l.PersistentResource = e.PersistentResource
			case "external":
				e.Workspace.Path = "/external"
			case "running":
				r.inspect = func(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
					return core.EnvironmentRuntimeStatus{State: core.EnvironmentRunning}, nil
				}
			case "unknown":
				r.inspect = func(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
					return core.EnvironmentRuntimeStatus{}, nil
				}
			case "drift":
				l.RuntimeRef = "other"
			case "recreated":
				s.identity = "env-22222222222222222222222222222222"
			case "unowned":
				l.Owner = ""
			case "resource-drift":
				e.PersistentResource = core.PersistentResourceRef{ID: "oci:one", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			case "resource-invalid":
				e.PersistentResource = core.PersistentResourceRef{ID: "oci:one"}
				l.PersistentResource = e.PersistentResource
			case "inspection-failure":
				r.inspect = func(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
					return core.EnvironmentRuntimeStatus{}, errors.New("unavailable")
				}
			}
			s.leases["resume"] = l
			s.environments["resume"] = e
			called := false
			err := New(r, s).withSnapshotSource(context.Background(), "resume", func(_ context.Context, source core.SnapshotSource) error {
				called = true
				if source.InstanceID != s.identity || source.Environment.PersistentResource != e.PersistentResource {
					t.Fatal("aggregate lost")
				}
				return nil
			})
			if (mode == "valid" || mode == "valid-resource") && (err != nil || !called) {
				t.Fatal(err)
			}
			if mode != "valid" && mode != "valid-resource" && (err == nil || called) {
				t.Fatal("unsafe capture admitted", mode, err)
			}
		})
	}
}
func TestSnapshotOperationKeepsLifecycleLock(t *testing.T) {
	s, r := snapshotFixture()
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- New(r, s).withSnapshotSource(context.Background(), "resume", func(context.Context, core.SnapshotSource) error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	err := New(r, s).Delete(ctx, "resume")
	cancel()
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if !errors.Is(err, context.DeadlineExceeded) || len(r.deleteRefs) != 0 {
		t.Fatal("delete overtook snapshot", err)
	}
}
