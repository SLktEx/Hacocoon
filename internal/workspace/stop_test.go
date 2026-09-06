package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type stoppingRuntime struct {
	fakeEnvironmentRuntime
	stopped   string
	stopError error
}

func (r *stoppingRuntime) StopEnvironment(_ context.Context, ref string) error {
	r.stopped = ref
	return r.stopError
}
func TestStopRetainsEnvironmentAndLeaseEvenOnFailure(t *testing.T) {
	for _, failure := range []error{nil, errors.New("stop uncertain")} {
		store := newFakeEnvironmentStore()
		environment := core.Environment{Name: "dev", RuntimeRef: "provider:exact", Workspace: core.Workspace{ID: "work", Path: "managed:work"}}
		lease := core.WorkspaceLease{WorkspaceID: "work", EnvironmentID: "dev", State: core.WorkspaceLeaseActive}
		store.environments["dev"] = environment
		store.leases["dev"] = lease
		runtime := &stoppingRuntime{stopError: failure}
		err := New(runtime, store).Stop(context.Background(), "dev")
		if !errors.Is(err, failure) || runtime.stopped != environment.RuntimeRef {
			t.Fatalf("stop=%s err=%v", runtime.stopped, err)
		}
		if !reflect.DeepEqual(store.environments["dev"], environment) || !reflect.DeepEqual(store.leases["dev"], lease) || len(runtime.deleteRefs) != 0 {
			t.Fatal("stop released or destroyed Workspace ownership")
		}
	}
}
