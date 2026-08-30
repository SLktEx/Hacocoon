package ec2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type permissiveArchiveRunner struct {
	fail bool
}

func (r *permissiveArchiveRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "tar" || len(args) < 2 || args[0] != "-czf" {
		return host.Result{}, errors.New("unexpected command")
	}
	if r.fail {
		return host.Result{}, errors.New("forced archive failure")
	}
	if err := os.WriteFile(args[1], []byte("archive"), 0o666); err != nil {
		return host.Result{}, err
	}
	if err := os.Chmod(args[1], 0o666); err != nil {
		return host.Result{}, err
	}
	return host.Result{}, nil
}

func TestCreateWorkspaceArchiveRemainsPrivateWithPermissiveArchiver(t *testing.T) {
	archive, cleanup, err := createWorkspaceArchive(context.Background(), &permissiveArchiveRunner{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	info, err := os.Lstat(archive)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("archive is not a regular file: mode=%s", info.Mode())
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("archive mode=%#o want=%#o", got, os.FileMode(0o600))
	}

	dir := filepath.Dir(archive)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("archive directory mode=%#o want=%#o", got, os.FileMode(0o700))
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("archive directory survived cleanup: %v", err)
	}
}

func TestCreateWorkspaceArchiveCleansPrivateDirectoryOnFailure(t *testing.T) {
	before, err := filepath.Glob(filepath.Join(os.TempDir(), "haco-workspace-*"))
	if err != nil {
		t.Fatal(err)
	}

	archive, cleanup, err := createWorkspaceArchive(context.Background(), &permissiveArchiveRunner{fail: true}, t.TempDir())
	if err == nil {
		t.Fatal("expected archive failure")
	}
	if archive != "" || cleanup != nil {
		t.Fatalf("failure returned archive=%q cleanup=%v", archive, cleanup != nil)
	}

	after, err := filepath.Glob(filepath.Join(os.TempDir(), "haco-workspace-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("temporary archive directory leaked: before=%d after=%d", len(before), len(after))
	}
}
