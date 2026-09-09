package controlapi

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmentcopy"
	"github.com/SLktEx/Hacocoon/internal/snapshotrestore"
	"strings"
	"testing"
)

type copyFunc func(context.Context, string, string) (environmentcopy.Result, error)

func (f copyFunc) CopyEnvironment(c context.Context, s, d string) (environmentcopy.Result, error) {
	return f(c, s, d)
}
func TestEnvironmentCopyWireRetainsCleanupIdentity(t *testing.T) {
	calls := 0
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterEnvironmentCopy(server, copyFunc(func(ctx context.Context, s, d string) (environmentcopy.Result, error) {
			calls++
			if s != "dev" || d != "" {
				t.Error(s, d)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Error("missing deadline")
			}
			return environmentcopy.Result{Result: snapshotrestore.Result{Environment: "dev-copy", State: "running"}, TemporarySnapshot: "snap-" + strings.Repeat("a", 32)}, core.ErrRecoveryRequired
		})); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(socket)
	result, err := client.CopyEnvironment(context.Background(), EnvironmentCopyRequest{Source: "dev"})
	if err == nil || result.Result.TemporarySnapshot == "" || result.Result.State != "running" {
		t.Fatal(result, err)
	}
	wire, _ := control.NewClient(control.UnixDialer(socket))
	for _, req := range []any{map[string]string{"source": "dev", "force": "true"}, map[string]string{"source": ""}} {
		if wire.Call(context.Background(), MethodEnvironmentCopy, req, nil) == nil {
			t.Fatal("unexpected input accepted")
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
