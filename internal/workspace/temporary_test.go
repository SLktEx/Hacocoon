package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"path/filepath"
	"testing"
)

func TestTemporaryWorkspaceUsesCanonicalLeaseAndRefusesRecycledName(t *testing.T) {
	ctx := context.Background()
	work, err := core.NewTemporaryWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "owned-temp"}}
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	service := New(runtime, store)
	env, err := service.Create(ctx, core.EnvironmentSpec{Name: "temporary", TemporaryWorkspace: &work})
	if err != nil {
		t.Fatal(err)
	}
	if env.Workspace != work || !runtime.createSpec.TemporaryWorkspace || runtime.createSpec.WorkspacePath != work.Path {
		t.Fatal("temporary source lost")
	}
	lease, err := store.GetWorkspaceLease(ctx, env.Name)
	if err != nil || lease.WorkspaceID != work.ID {
		t.Fatal("missing canonical lease")
	}
	other, _ := core.NewTemporaryWorkspace()
	if err := service.DeleteTemporary(ctx, env.Name, other); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
	if len(runtime.deleteRefs) != 0 {
		t.Fatal("deleted a different Workspace")
	}
	if err := service.DeleteTemporary(ctx, env.Name, work); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetWorkspaceLease(ctx, env.Name); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("lease retained after verified deletion")
	}
	if _, err := service.Create(ctx, core.EnvironmentSpec{Name: "temporary", WorkspacePath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteTemporary(ctx, "temporary", work); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("recycled name accepted", err)
	}
	if len(runtime.deleteRefs) != 1 {
		t.Fatal("recycled Environment was deleted")
	}
}
func TestTemporaryWorkspaceCannotAliasHostPath(t *testing.T) {
	service := New(&fakeEnvironmentRuntime{}, state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json")))
	for _, work := range []core.Workspace{{ID: "workspace:temporary:bad", Path: "/root"}, {ID: "workspace:temporary:bad", Path: "temporary:../../root"}} {
		if _, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "temp", TemporaryWorkspace: &work}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
}
