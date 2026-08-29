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

func (detachFailRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name == "qemu-nbd" && len(args) == 2 && args[0] == "--disconnect" {
		return host.Result{ExitCode: 1}, errors.New("forced detach failure")
	}
	return host.Result{}, nil
}

func TestDeletePreservesBackingFileWhenDetachFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.qcow2")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(detachFailRunner{})
	err := store.Delete(context.Background(), block.Handle{Path: path, Device: "/dev/nbd999"})
	if err == nil {
		t.Fatal("Delete succeeded despite detach failure")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("backing file was removed after detach failure: %v", statErr)
	}
}
