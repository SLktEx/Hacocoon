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
	spec := core.StorageSpec{ID: id, SizeBytes: 512 << 20}
	handle, err := storage.Ensure(ctx, spec)
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

	repeated, err := storage.Ensure(ctx, spec)
	if err != nil {
		t.Fatalf("repeat managed Btrfs ensure through helper: %v", err)
	}
	if repeated.ID != handle.ID || repeated.Attachment["driver"] != handle.Attachment["driver"] || repeated.Attachment["source"] != handle.Attachment["source"] || repeated.Attachment["incus_pool"] != handle.Attachment["incus_pool"] {
		t.Fatalf("repeated ensure changed attachment: first=%#v repeated=%#v", handle, repeated)
	}

	source := handle.Attachment["source"]
	if source != filepath.Join(root, "mounts", id) {
		t.Fatalf("managed source = %q, want %q", source, filepath.Join(root, "mounts", id))
	}
	if handle.Attachment["driver"] != "btrfs" {
		t.Fatalf("managed driver = %q, want btrfs", handle.Attachment["driver"])
	}
	backing := filepath.Join(root, "images", id+".raw")

	// Reproduce the partial state seen on real WSL: the sparse image remains,
	// its loop device remains attached and already formatted as Btrfs, but the
	// managed mount itself is absent. A normal Ensure must restore the mount
	// rather than handing the underlying ext4 directory to Incus.
	unmountResult, err := privileged.Run(ctx, "umount", source)
	if err != nil {
		t.Fatalf("remove managed mount for recovery fixture: result=%#v err=%v", unmountResult, err)
	}
	mounted, mountErr := direct.Run(ctx, "findmnt", "-rn", "--mountpoint", source)
	if mountErr == nil || mounted.ExitCode != 1 {
		t.Fatalf("managed mount still present in recovery fixture: result=%#v err=%v", mounted, mountErr)
	}
	loop, err := privileged.Run(ctx, "losetup", "-j", backing)
	if err != nil || strings.TrimSpace(loop.Stdout) == "" {
		t.Fatalf("managed loop disappeared from recovery fixture: result=%#v err=%v", loop, err)
	}
	recovered, err := storage.Ensure(ctx, spec)
	if err != nil {
		t.Fatalf("recover missing managed Btrfs mount through helper: %v", err)
	}
	if recovered.ID != handle.ID || recovered.Attachment["source"] != source || recovered.Attachment["incus_pool"] != handle.Attachment["incus_pool"] {
		t.Fatalf("recovered ensure changed attachment: first=%#v recovered=%#v", handle, recovered)
	}

	state, err := storage.Inspect(ctx, handle)
	if err != nil {
		t.Fatalf("inspect managed Btrfs storage: %v", err)
	}
	if !state.Healthy || state.LogicalBytes <= 0 {
		t.Fatalf("unexpected managed storage state: %#v", state)
	}

	fstype, err := direct.Run(ctx, "findmnt", "-rn", "-o", "FSTYPE", "--mountpoint", source)
	if err != nil || strings.TrimSpace(fstype.Stdout) != "btrfs" {
		t.Fatalf("managed mount filesystem = %q err=%v, want btrfs", strings.TrimSpace(fstype.Stdout), err)
	}
	options, err := direct.Run(ctx, "findmnt", "-rn", "-o", "OPTIONS", "--mountpoint", source)
	if err != nil || !strings.Contains(strings.TrimSpace(options.Stdout), "compress=zstd:3") {
		t.Fatalf("managed mount options = %q err=%v, want compress=zstd:3", strings.TrimSpace(options.Stdout), err)
	}

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
	mounted, mountErr = direct.Run(ctx, "findmnt", "-rn", "--mountpoint", source)
	if mountErr == nil || mounted.ExitCode != 1 {
		t.Fatalf("managed mount remains after delete: result=%#v err=%v", mounted, mountErr)
	}
}
