package controlapi

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/basemanage"
	"github.com/SLktEx/Hacocoon/internal/control"
	"strings"
	"testing"
)

type baseManageFixture struct{ deleted []basemanage.Identity }

func (f *baseManageFixture) List(context.Context) ([]basemanage.Image, error) {
	return []basemanage.Image{}, nil
}
func (f *baseManageFixture) Delete(ctx context.Context, id basemanage.Identity) error {
	if _, ok := ctx.Deadline(); !ok {
		panic("missing deadline")
	}
	f.deleted = append(f.deleted, id)
	return nil
}
func TestBaseManageWireRejectsExtraAuthorityAndPreservesIdentity(t *testing.T) {
	f := &baseManageFixture{}
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterBaseManage(server, f); err != nil {
			t.Fatal(err)
		}
	})
	c, _ := NewClient(socket)
	ctx := context.Background()
	id := basemanage.Identity{Name: "tools", Fingerprint: strings.Repeat("a", 64), BuildInstance: "env-" + strings.Repeat("b", 32)}
	if err := c.DeleteBaseImage(ctx, id); err != nil || len(f.deleted) != 1 || f.deleted[0] != id {
		t.Fatal(err, f.deleted)
	}
	wire, _ := control.NewClient(control.UnixDialer(socket))
	for _, req := range []any{map[string]any{"operation": "delete", "image": id, "force": true}, BaseManageRequest{Operation: "list", Image: id}, BaseManageRequest{Operation: "delete"}} {
		if wire.Call(ctx, MethodBaseManage, req, nil) == nil {
			t.Fatal("invalid accepted", req)
		}
	}
	if len(f.deleted) != 1 {
		t.Fatal(f.deleted)
	}
}
