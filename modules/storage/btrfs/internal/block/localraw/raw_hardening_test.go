package localraw

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type hardeningRunner struct{}

func (hardeningRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	joined := name + " " + strings.Join(args, " ")
	switch {
	case strings.HasPrefix(joined, "losetup -j "):
		return host.Result{Stdout: "/dev/loop7: []: (image)\n"}, nil
	case joined == "losetup -d /dev/loop7":
		return host.Result{ExitCode: 1}, errors.New("device busy")
	default:
		return host.Result{}, errors.New("unexpected command: " + joined)
	}
}

func TestDeleteKeepsBackingFileWhenLoopDetachFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.raw")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(hardeningRunner{})
	if err := store.Delete(context.Background(), block.Handle{ID: "test", Path: path}); err == nil {
		t.Fatal("expected detach failure")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("backing file was removed after detach failure: %v", err)
	}
}
