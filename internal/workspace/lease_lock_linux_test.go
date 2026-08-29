//go:build linux

package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureTrustedWorkspaceLockDirectoryRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "locks")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := ensureTrustedWorkspaceLockDirectory(link); err == nil {
		t.Fatal("expected symlink lock directory to be rejected")
	}
}

func TestEnsureTrustedWorkspaceLockDirectoryRejectsUnsafeMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensureTrustedWorkspaceLockDirectory(dir); err == nil {
		t.Fatal("expected group/world-accessible lock directory to be rejected")
	}
}

func TestEnsureTrustedWorkspaceLockDirectoryAcceptsPrivateOwnedDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	if err := ensureTrustedWorkspaceLockDirectory(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("info=%v", info.Mode())
	}
}
