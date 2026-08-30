//go:build linux

package localqcow2

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

func TestRealNBDIdentityRecoveryGate(t *testing.T) {
	if os.Getenv("HACO_REAL_NBD_E2E") != "1" {
		t.Skip("set HACO_REAL_NBD_E2E=1 on a disposable root-capable Linux host with qemu-nbd/NBD devices")
	}
	if os.Geteuid() != 0 {
		t.Fatal("HACO_REAL_NBD_E2E requires root so qemu-nbd can attach a real NBD device")
	}

	ctx := context.Background()
	runner := host.ExecRunner{}
	store := New(runner)
	caps, err := store.Probe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Available {
		t.Fatalf("real NBD gate requested but local-qcow2 is unavailable: %v", caps.Details)
	}

	root := filepath.Join(t.TempDir(), "storage")
	backing := filepath.Join(root, "images", "acceptance.qcow2")
	if err := block.PrepareBackingDirectory(backing); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(ctx, "qemu-img", "create", "-f", "qcow2", backing, "64M"); err != nil {
		t.Fatal(err)
	}

	attached, err := store.Attach(ctx, block.Handle{ID: "acceptance", Path: backing, Bytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			_ = store.Detach(context.Background(), attached)
		}
	}()

	// Construct a fresh Store with no in-memory Handle.Device. This simulates
	// the upper layer reconstructing the block handle after a process restart.
	restarted := New(runner)
	state, err := restarted.Inspect(ctx, block.Handle{ID: "acceptance", Path: backing, Bytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if state.Device == "" || state.Device != attached.Device {
		t.Fatalf("restart did not reconstruct the exact NBD mapping: attached=%q observed=%q", attached.Device, state.Device)
	}

	if err := restarted.Delete(ctx, block.Handle{ID: "acceptance", Path: backing, Bytes: 64 << 20}); err != nil {
		t.Fatal(err)
	}
	cleanupNeeded = false
	if _, err := os.Stat(backing); !os.IsNotExist(err) {
		t.Fatalf("verified delete left backing image behind: %v", err)
	}
	if _, err := os.Stat(nbdStatePath(backing)); !os.IsNotExist(err) {
		t.Fatalf("verified delete left NBD identity state behind: %v", err)
	}
}
