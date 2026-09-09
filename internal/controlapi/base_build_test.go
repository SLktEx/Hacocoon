package controlapi

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type buildFunc func(context.Context, basebuild.Definition) (basebuild.Result, error)

func (f buildFunc) Build(ctx context.Context, d basebuild.Definition) (basebuild.Result, error) {
	return f(ctx, d)
}
func TestBaseBuildWirePreservesPublicationEvidence(t *testing.T) {
	calls := 0
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterBaseBuild(server, buildFunc(func(ctx context.Context, d basebuild.Definition) (basebuild.Result, error) {
			calls++
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("no deadline")
			}
			return basebuild.Result{Builder: "build-test", State: "publication-unconfirmed"}, core.ErrRecoveryRequired
		})); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(socket)
	got, err := client.BuildBase(context.Background(), basebuild.Definition{Name: "tools", Run: "true"})
	if err == nil || got.Result.Builder != "build-test" {
		t.Fatal(got, err)
	}
	wire, _ := control.NewClient(control.UnixDialer(socket))
	for _, req := range []any{map[string]string{"name": "tools", "run": "true", "privileged": "true"}, map[string]string{"name": "--public", "run": "true"}} {
		if wire.Call(context.Background(), MethodBaseBuild, req, nil) == nil {
			t.Fatal("invalid accepted")
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
