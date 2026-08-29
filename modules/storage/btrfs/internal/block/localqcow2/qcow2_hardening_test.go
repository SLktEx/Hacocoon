package localqcow2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type detachFailRunner struct{}

func (detachFailRunner) Run(context.Context, string, ...string) (host.Result, error) {
	return host.Result{ExitCode: -1}, errors.New("synthetic qemu-nbd failure")
}

func TestDeleteRefusesQCOW2WithoutDeviceIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.qcow2")
	if err := os.WriteFile(path, []byte("important"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := New(detachFailRunner{})
	if err := store.Delete(context.Background(), block.Handle{ID: "demo", Path: path}); err == nil {
		t.Fatal("expected delete to fail without an NBD device identity")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("qcow2 image was removed without a device identity: %v", err)
	}
}

func TestDeletePreservesQCOW2WhenDisconnectFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.qcow2")
	if err := os.WriteFile(path, []byte("important"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := New(detachFailRunner{})
	if err := store.Delete(context.Background(), block.Handle{ID: "demo", Path: path, Device: "/dev/nbd7"}); err == nil {
		t.Fatal("expected delete to fail when qemu-nbd disconnect fails")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("qcow2 image was removed after failed disconnect: %v", err)
	}
}
