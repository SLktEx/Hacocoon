package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestSSHMigrationRemovesOnlyVerifiedLegacySSHDevice(t *testing.T) {
	r := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if args[0] == "query" {
			return host.Result{Stdout: `{"devices":{"haco-ssh-2222":{"type":"proxy","listen":"tcp:127.0.0.1:2222","connect":"tcp:127.0.0.1:22"},"haco-tcp-8080-3000":{"type":"proxy","listen":"tcp:127.0.0.1:8080","connect":"tcp:127.0.0.1:3000"}}}`}, nil
		}
		return host.Result{}, nil
	}}
	if err := New(r).MigrateSSHAccess(context.Background(), "haco-demo"); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 4 || r.calls[2].args[0] != "exec" {
		t.Fatalf("revoke must precede removal: %+v", r.calls)
	}
	last := strings.Join(r.calls[3].args, " ")
	if !strings.Contains(last, "device remove") || !strings.Contains(last, "haco-ssh-2222") || strings.Contains(last, "8080") {
		t.Fatal(last)
	}
}
