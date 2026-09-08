package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

func TestCreateDefaultResourceAndOptOutUseCanonicalLease(t *testing.T) {
	for _, skip := range []bool{false, true} {
		runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-default"}}
		store := newFakeEnvironmentStore()
		svc := New(runtime, store)
		calls := 0
		svc.ConfigureDefaultResource(func(_ context.Context, w core.Workspace) (core.PersistentResource, error) {
			calls++
			return core.PersistentResource{ID: "oci:auto-dev", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", State: "ready", WorkspaceID: w.ID}, nil
		})
		env, err := svc.Create(context.Background(), core.EnvironmentSpec{Name: "default", WorkspacePath: t.TempDir(), SkipDefaultResource: skip})
		if err != nil {
			t.Fatal(err)
		}
		lease := store.leases["default"]
		if skip {
			if calls != 0 || env.PersistentResource != (core.PersistentResourceRef{}) {
				t.Fatal("opt-out initialized OCI")
			}
		} else {
			if calls != 1 || env.PersistentResource.ID != "oci:auto-dev" || lease.PersistentResource != env.PersistentResource || runtime.createSpec.PersistentResource.Ref() != env.PersistentResource {
				t.Fatal("default resource bypassed aggregate")
			}
		}
	}
}
func TestDefaultResourceFailureNeverStartsAnEmptyEnvironment(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{}
	svc := New(runtime, newFakeEnvironmentStore())
	svc.ConfigureDefaultResource(func(context.Context, core.Workspace) (core.PersistentResource, error) {
		return core.PersistentResource{}, core.ErrRecoveryRequired
	})
	_, err := svc.Create(context.Background(), core.EnvironmentSpec{Name: "default", WorkspacePath: t.TempDir()})
	if !errors.Is(err, core.ErrRecoveryRequired) || runtime.createSpec != (core.EnvironmentRuntimeSpec{}) {
		t.Fatalf("%v %+v", err, runtime.createSpec)
	}
}
