package localraw

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type failingRunner struct {
	calls int
}

func (r *failingRunner) Run(context.Context, string, ...string) (host.Result, error) {
	r.calls++
	return host.Result{ExitCode: -1}, errors.New("synthetic losetup failure")
}

func TestDeletePreservesRawImageWhenLoopLookupFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.raw")
	if err := os.WriteFile(path, []byte("important"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &failingRunner{}
	store := New(runner)
	err := store.Delete(context.Background(), block.Handle{ID: "demo", Path: path})
	if err == nil {
		t.Fatal("expected delete to fail closed when loop lookup fails")
	}
	if runner.calls != 1 {
		t.Fatalf("unexpected runner calls: got %d want 1", runner.calls)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("raw image was removed after failed loop lookup: %v", statErr)
	}
}
