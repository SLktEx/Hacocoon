package localraw

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

func TestRawEnsureRejectsHardlinkedBackingImageBeforeResize(t *testing.T) {
	path := trustedRawPath(t)
	victim := filepath.Join(filepath.Dir(filepath.Dir(path)), "victim")
	want := []byte("do-not-truncate")
	if err := os.WriteFile(victim, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(victim, path); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	runner := &recordingRunner{}
	if _, err := New(runner).Ensure(context.Background(), block.Spec{ID: "demo", Path: path, SizeBytes: 1}); err == nil {
		t.Fatal("Ensure accepted hardlinked backing image")
	}
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("hardlink victim changed: got %q want %q", got, want)
	}
	if runner.calls != 0 {
		t.Fatalf("privileged runner invoked after hardlink rejection: %d calls", runner.calls)
	}
}
