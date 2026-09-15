//go:build linux

package workflow

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readyReference() PathReference {
	return PathReference{Version: 1, Reference: Reference{Name: "work", Workspace: "workspace:managed:owner"}, State: "ready", OCI: "none"}
}
func TestReferenceSurvivesReopenWithoutTouchingDirectoryFiles(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	if err := os.WriteFile(original, []byte("untouched"), 0644); err != nil {
		t.Fatal(err)
	}
	h, err := LockReference(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	ref := readyReference()
	if err = h.Save(ref); err != nil {
		t.Fatal(err)
	}
	h.Close()
	h, err = LockReference(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	got, err := h.Load()
	if err != nil || got.Reference != ref.Reference || got.OCI != "none" {
		t.Fatal(got, err)
	}
	raw, _ := os.ReadFile(original)
	if string(raw) != "untouched" {
		t.Fatal("modified original")
	}
	info, _ := os.Stat(filepath.Join(dir, ReferenceFile))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}
func TestReferenceRefusesSymlinksHardlinksFIFOsAndBroadPermissions(t *testing.T) {
	for _, entry := range []string{ReferenceFile, referenceLock} {
		for _, mode := range []string{"symlink", "hardlink", "fifo", "permissions"} {
			t.Run(entry+"/"+mode, func(t *testing.T) {
				dir := t.TempDir()
				outside := filepath.Join(t.TempDir(), "keep")
				if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(dir, entry)
				var err error
				switch mode {
				case "symlink":
					err = os.Symlink(outside, target)
				case "hardlink":
					err = os.Link(outside, target)
				case "fifo":
					err = unix.Mkfifo(target, 0600)
				case "permissions":
					err = os.WriteFile(target, []byte("{}"), 0644)
				}
				if err != nil {
					t.Fatal(err)
				}
				h, err := LockReference(context.Background(), dir)
				if entry == referenceLock {
					if err == nil {
						h.Close()
						t.Fatal("unsafe lock accepted")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					defer h.Close()
					if _, err = h.Load(); err == nil {
						t.Fatal("unsafe read")
					}
					if err = h.Save(readyReference()); err == nil {
						t.Fatal("unsafe replacement")
					}
				}
				raw, _ := os.ReadFile(outside)
				if string(raw) != "untouched" {
					t.Fatal("outside changed")
				}
			})
		}
	}
}
func TestReferenceLockCancellationAndPinnedDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "work")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	h, err := LockReference(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if second, err := LockReference(ctx, dir); !errors.Is(err, context.DeadlineExceeded) {
		if second != nil {
			second.Close()
		}
		t.Fatal(err)
	}
	moved := filepath.Join(parent, "moved")
	if err = os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err = h.Save(readyReference()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, ReferenceFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("retargeted renamed directory", err)
	}
	if _, err = os.Stat(filepath.Join(moved, ReferenceFile)); err != nil {
		t.Fatal(err)
	}
}
