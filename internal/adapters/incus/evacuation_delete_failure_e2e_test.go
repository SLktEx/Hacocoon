package incus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// Only the freshly created fixture snapshot parent is made immutable. This models
// a real EPERM from Btrfs deletion, not filesystem corruption or arbitrary damage.
func blockOwnedSnapshotDeletion(t *testing.T, ctx context.Context, runner host.ExecRunner, snapshot, ledger string) func() bool {
	t.Helper()
	parent := filepath.Dir(snapshot)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("owned snapshot parent unavailable", err)
	}
	// Preserve the exact path and identity before changing its attributes. If the
	// process dies, the existing pool owner plan and this record permit review.
	data, err := json.Marshal(map[string]any{"path": parent, "identity": info.Sys(), "cleanup": "clear immutable only after verifying the fixture pool owner and directory identity"})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(ledger, "immutable-"+filepath.Base(parent)+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(ledger)
	if err != nil {
		t.Fatal(err)
	}
	err = directory.Sync()
	directory.Close()
	if err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() bool {
		if released {
			return true
		}
		now, err := os.Lstat(parent)
		if err != nil || !os.SameFile(info, now) || now.Mode()&os.ModeSymlink != 0 {
			t.Errorf("refusing immutable cleanup after directory replacement: %s", parent)
			return false
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		out, err := runner.Run(cleanup, "chattr", "-i", parent)
		if err != nil || out.ExitCode != 0 {
			t.Errorf("immutable cleanup unresolved; retain ownership ledger for %s", parent)
			return false
		}
		released = true
		return true
	}
	t.Cleanup(func() { release() })
	out, err := runner.Run(ctx, "chattr", "+i", parent)
	if err != nil || out.ExitCode != 0 {
		t.Fatal("cannot inject immutable-parent deletion failure", err)
	}
	return release
}
