//go:build linux

package incus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAggregateRecoveryCleanupRetainsCatalogLocksAndRejectsUnknownObjects(t *testing.T) {
	for _, kind := range []string{"files", "locks", "unknown-directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "fixture")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			receipt := filepath.Join(dir, "receipt.json")
			if err := os.WriteFile(receipt, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			locks := filepath.Join(dir, "lifecycle-locks")
			switch kind {
			case "locks":
				if err := os.Mkdir(locks, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(locks, "identity.lock"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "unknown-directory":
				if err := os.Mkdir(filepath.Join(dir, "unknown"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(t.TempDir(), locks); err != nil {
					t.Fatal(err)
				}
			}
			retained, err := cleanupAggregateRecoveryFiles(dir)
			if kind == "unknown-directory" || kind == "symlink" {
				if err == nil {
					t.Fatal("unexpected object accepted")
				}
				if _, err := os.Stat(receipt); err != nil {
					t.Fatal("receipt lost on refused cleanup", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if retained != (kind == "locks") {
				t.Fatal("incorrect retention result")
			}
			if kind == "locks" {
				if _, err := os.Stat(filepath.Join(locks, "identity.lock")); err != nil {
					t.Fatal("lock identity removed", err)
				}
			} else if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatal("file-only fixture remains", err)
			}
		})
	}
}
