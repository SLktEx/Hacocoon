package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type storagePoolErrorRunner struct{}

func (storagePoolErrorRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "incus" || len(args) < 2 || args[0] != "storage" {
		return host.Result{ExitCode: -1}, errors.New("unexpected command")
	}
	switch args[1] {
	case "show":
		return host.Result{ExitCode: 1}, errors.New("exit status 1")
	case "create":
		return host.Result{
			ExitCode: 1,
			Stderr:   "Error: Provided path does not reside on a btrfs filesystem (detected ext4)\n",
		}, errors.New("exit status 1")
	default:
		return host.Result{ExitCode: -1}, errors.New("unexpected storage command")
	}
}

func TestEnsureStoragePoolSurfacesCreateStderr(t *testing.T) {
	runtime := New(storagePoolErrorRunner{})
	pool, err := runtime.ensureStoragePool(context.Background(), map[string]string{
		"incus_pool": "haco-local-default",
		"driver":     "btrfs",
		"source":     "/var/lib/hacocoon/mounts/local-default",
	})
	if pool != "" {
		t.Fatalf("pool = %q, want empty on create failure", pool)
	}
	if err == nil {
		t.Fatal("expected storage pool creation failure")
	}
	for _, want := range []string{
		"haco-local-default",
		"/var/lib/hacocoon/mounts/local-default",
		"Provided path does not reside on a btrfs filesystem (detected ext4)",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}
