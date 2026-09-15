package controlapi

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/base/manage"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type baseManageFixture struct {
	deleted []basemanage.Identity
	images  []basemanage.Image
	failure error
}

func (f *baseManageFixture) List(context.Context) ([]basemanage.Image, error) {
	return f.images, f.failure
}

func TestBaseImageListWirePreservesProtectionAndSavedCopies(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(strconv.FormatBool(failed), func(t *testing.T) {
			images := []basemanage.Image{{Identity: basemanage.Identity{Name: "tools", Fingerprint: strings.Repeat("a", 64), BuildInstance: "env-" + strings.Repeat("b", 32)}, Current: true, Aliases: []string{"tools"}, NativeUsers: []string{"native-user"}, ProtectedAliases: []string{"ubuntu-stable"}, Environments: []string{"dev"}, IndependentSnapshots: []string{"saved-copy"}}}
			f := &baseManageFixture{images: images}
			if failed {
				f.failure = core.ErrRuntimeUnavailable
			}
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterBaseManage(s, f); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.ListBaseImages(context.Background())
			if failed {
				var status *control.StatusError
				if !errors.As(err, &status) || status.Code != "unavailable" {
					t.Fatal("incomplete catalog reported as successful", got, err)
				}
			} else if err != nil || !reflect.DeepEqual(got, images) {
				t.Fatal("image review lost protection or copies", got, err)
			}
			if len(f.deleted) != 0 {
				t.Fatal("list mutated Base images", f.deleted)
			}
		})
	}
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
