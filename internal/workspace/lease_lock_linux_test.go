//go:build linux

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestEnsureTrustedWorkspaceLockDirectoryCreatesSecureDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	if err := ensureTrustedWorkspaceLockDirectory(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %v", info.Mode())
	}
	if err := validateTrustedWorkspaceLockDirectory(dir, info, uint32(os.Geteuid())); err != nil {
		t.Fatalf("created directory was not trusted: %v", err)
	}
}

func TestEnsureTrustedWorkspaceLockDirectoryRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "locks")
	if err := os.Symlink(target, dir); err != nil {
		t.Fatal(err)
	}
	if err := ensureTrustedWorkspaceLockDirectory(dir); err == nil {
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
		t.Fatal("expected unsafe permissions to be rejected")
	}
}

func TestValidateTrustedWorkspaceLockDirectoryRejectsWrongOwner(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	wrongUID := uint32(os.Geteuid()) ^ 1
	if err := validateTrustedWorkspaceLockDirectory(dir, info, wrongUID); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("error = %v", err)
	}
}

func TestLockWorkspaceCreatesOwnerOnlyLockFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)

	unlock, err := lockWorkspace(context.Background(), core.WorkspaceID("workspace-1"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	matches, err := filepath.Glob(filepath.Join(root, "hacocoon-workspace-locks", "*.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("lock files = %#v", matches)
	}
	info, err := os.Stat(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("lock file mode = %o", info.Mode().Perm())
	}
}
