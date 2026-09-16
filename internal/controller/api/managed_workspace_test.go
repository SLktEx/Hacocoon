package controlapi

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"reflect"
	"strconv"
	"testing"
)

type managedWorkspaceFixture struct {
	calls      int
	received   core.Workspace
	workspaces []workspace.ManagedWorkspace
	failure    error
}

func (f *managedWorkspaceFixture) ListManagedWorkspaces(context.Context) ([]workspace.ManagedWorkspace, error) {
	return f.workspaces, f.failure
}

func TestManagedWorkspaceListWirePreservesOwnersAndRetainedReferences(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(strconv.FormatBool(failed), func(t *testing.T) {
			items := []workspace.ManagedWorkspace{{Workspace: core.Workspace{ID: "workspace:managed:owner", Path: "managed:work"}, Name: "work", State: "ready", Repositories: []string{"source"}, Environments: []string{"dev"}, Snapshots: []string{"saved"}, Stores: []string{"oci:retained"}}}
			f := &managedWorkspaceFixture{workspaces: items}
			if failed {
				f.failure = core.ErrRecoveryRequired
			}
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterManagedWorkspaces(s, f); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.ListManagedWorkspaces(context.Background())
			if failed {
				var status *control.StatusError
				if !errors.As(err, &status) || status.Code != "recovery_required" {
					t.Fatal("workspace recovery hidden", got, err)
				}
			} else if err != nil || !reflect.DeepEqual(got, items) {
				t.Fatal("workspace review lost ownership/references", got, err)
			}
			if f.calls != 0 {
				t.Fatal("list deleted a Workspace", f.calls)
			}
		})
	}
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
