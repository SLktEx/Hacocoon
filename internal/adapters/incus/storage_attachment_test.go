package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestStorageAttachmentRejectsExternalSourceAndInvalidIdentity(t *testing.T) {
	for _, attachment := range []map[string]string{
		{"incus_pool": "haco-local-default", "driver": "btrfs", "source": "/var/lib/hacocoon/mounts/local-default"},
		{"source": "/tmp/external"},
		{"incus_pool": ""},
		{"incus_pool": "--help"},
		{"incus_pool": "remote:pool"},
		{"incus_pool": "../pool"},
		{"incus_pool": "pool\n"},
	} {
		runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
			t.Fatal("invalid attachment reached Incus")
			return host.Result{}, nil
		}}
		pool, err := New(runner).ensureStoragePool(context.Background(), attachment)
		if pool != "" || !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("attachment=%v: pool=%q, error=%v", attachment, pool, err)
		}
	}
}

func TestStorageAttachmentUnavailableDoesNotCreateReplacement(t *testing.T) {
	failure := errors.New("pool inspection failed")
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" || len(args) != 5 || args[0] != "storage" || args[1] != "show" || args[2] != "haco-local-default" {
			t.Fatalf("unexpected storage mutation: %s %v", name, args)
		}
		return host.Result{}, failure
	}}
	pool, err := New(runner).ensureStoragePool(context.Background(), map[string]string{"incus_pool": "haco-local-default"})
	if pool != "" || !errors.Is(err, failure) || len(runner.calls) != 1 {
		t.Fatalf("pool=%q, error=%v, calls=%v", pool, err, runner.calls)
	}
}
