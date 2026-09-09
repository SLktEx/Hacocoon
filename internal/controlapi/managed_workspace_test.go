package controlapi

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"testing"
)

type managedWorkspaceFixture struct {
	calls    int
	received core.Workspace
}

func (f *managedWorkspaceFixture) ListManagedWorkspaces(context.Context) ([]workspace.ManagedWorkspace, error) {
	return []workspace.ManagedWorkspace{}, nil
}
func (f *managedWorkspaceFixture) DeleteManagedWorkspace(ctx context.Context, w core.Workspace) error {
	f.calls++
	f.received = w
	if _, ok := ctx.Deadline(); !ok {
		panic("unbounded deletion")
	}
	return core.ErrCapabilityStale
}
func TestManagedWorkspaceWireRejectsUnknownFieldsAndPreservesOwner(t *testing.T) {
	f := &managedWorkspaceFixture{}
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterManagedWorkspaces(server, f); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(socket)
	if err != nil {
		t.Fatal(err)
	}
	w := core.Workspace{ID: "workspace:managed:old", Path: "managed:work"}
	if err := client.DeleteManagedWorkspace(context.Background(), w); err == nil || f.calls != 1 || f.received != w {
		t.Fatal("stale owner accepted", err, f)
	}
	wire, err := control.NewClient(control.UnixDialer(socket))
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []any{map[string]any{"operation": "delete", "workspace": w, "force": true}, ManagedWorkspaceRequest{Operation: "delete"}, ManagedWorkspaceRequest{Operation: "list", Workspace: w}} {
		if wire.Call(context.Background(), MethodManagedWorkspace, req, nil) == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if f.calls != 1 {
		t.Fatal("invalid request reached deletion")
	}
}
