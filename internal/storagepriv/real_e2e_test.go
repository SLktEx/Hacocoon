package storagepriv_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/storagepriv"
	storagebtrfs "github.com/SLktEx/Hacocoon/modules/storage/btrfs"
)

func TestRealPrivilegedStorageHelperE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_STORAGE_HELPER") != "1" {
		t.Skip("set HACO_E2E_STORAGE_HELPER=1 on an isolated Linux host to run the real privileged storage helper acceptance")
	}
	if os.Geteuid() == 0 {
		t.Fatal("real storage helper acceptance must run the test process as an ordinary user")
	}
	root := strings.TrimSpace(os.Getenv("HACO_E2E_STORAGE_ROOT"))
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		t.Fatalf("HACO_E2E_STORAGE_ROOT must be an absolute clean path, got %q", root)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("create E2E storage root: %v", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatalf("lock E2E storage root permissions: %v", err)
	}

	ctx := context.Background()
	direct := host.ExecRunner{}
	privileged, err := storagepriv.NewSudoRunner(root, direct)
	if err != nil {
		t.Fatalf("create privileged storage runner: %v", err)
	}
	storage, err := storagebtrfs.NewLocal(ctx, root, privileged, "raw")
	if err != nil {
		t.Fatalf("compose managed Btrfs storage: %v", err)
	}

	const id = "helper-e2e"
	handle, err := storage.Ensure(ctx, core.StorageSpec{ID: id, SizeBytes: 512 << 20})
	if err != nil {
		t.Fatalf("ensure managed Btrfs storage through helper: %v", err)
	}
	cleaned := false
	t.Cleanup(func() {
		if cleaned {
			return
		}
		if err := storage.Delete(context.Background(), handle); err != nil {
			t.Errorf("cleanup managed Btrfs storage: %v", err)
		}
	})

	source := handle.Attachment["source"]
	if source != filepath.Join(root, "mounts", id) {
		t.Fatalf("managed source = %q, want %q", source, filepath.Join(root, "mounts", id))
	}
	if handle.Attachment["driver"] != "btrfs" {
		t.Fatalf("managed driver = %q, want btrfs", handle.Attachment["driver"])
	}

	state, err := storage.Inspect(ctx, handle)
	if err != nil {
		t.Fatalf("inspect managed Btrfs storage: %v", err)
	}
	if !state.Healthy || state.LogicalBytes <= 0 {
		t.Fatalf("unexpected managed storage state: %#v", state)
	}

	fstype, err := direct.Run(ctx, "findmnt", "-rn", "-o", "FSTYPE", "--target", source)
	if err != nil || strings.TrimSpace(fstype.Stdout) != "btrfs" {
		t.Fatalf("managed mount filesystem = %q err=%v, want btrfs", strings.TrimSpace(fstype.Stdout), err)
	}
	options, err := direct.Run(ctx, "findmnt", "-rn", "-o", "OPTIONS", "--target", source)
	if err != nil || !strings.Contains(strings.TrimSpace(options.Stdout), "compress=zstd:3") {
		t.Fatalf("managed mount options = %q err=%v, want compress=zstd:3", strings.TrimSpace(options.Stdout), err)
	}

	backing := filepath.Join(root, "images", id+".raw")
	info, err := os.Lstat(backing)
	if err != nil {
		t.Fatalf("inspect sparse backing image: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		t.Fatalf("unexpected sparse backing image mode: %s", info.Mode())
	}

	if err := storage.Delete(ctx, handle); err != nil {
		t.Fatalf("delete managed Btrfs storage through helper: %v", err)
	}
	cleaned = true
	if _, err := os.Lstat(backing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backing image remains after delete: %v", err)
	}
	mounted, mountErr := direct.Run(ctx, "findmnt", "-rn", "--target", source)
	if mountErr == nil || mounted.ExitCode != 1 {
		t.Fatalf("managed mount remains after delete: result=%#v err=%v", mounted, mountErr)
	}
}
