//go:build linux

package state

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLifecycleLockIgnoresSharedTemporaryNames(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	for _, domain := range []string{"environment", "workspace"} {
		// An unusable old global name must neither block the catalog nor be repaired.
		path := filepath.Join(tmp, "hacocoon-"+domain+"-locks")
		if err := os.WriteFile(path, []byte("foreign"), 0o600); err != nil {
			t.Fatal(err)
		}
		catalog := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
		unlock, err := catalog.LockLifecycle(context.Background(), domain, "dev")
		if err != nil {
			t.Fatal(err)
		}
		unlock()
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "foreign" {
			t.Fatal("shared temporary object changed", err)
		}
		files, err := filepath.Glob(filepath.Join(filepath.Dir(catalog.path), "lifecycle-locks", "lifecycle-*.lock"))
		if err != nil || len(files) != 1 {
			t.Fatal(files, err)
		}
		info, err := os.Lstat(files[0])
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatal("unsafe lock file", err)
		}
	}
}

func TestLifecycleLockCatalogScopeAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "environments.json")
	first := NewEnvironmentJSONStore(path)
	unlock, err := first.LockLifecycle(context.Background(), "environment", "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	second := NewEnvironmentJSONStore(path)
	if release, err := second.LockLifecycle(ctx, "environment", "dev"); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatalf("same catalog did not exclude: %v", err)
	}
	for _, candidate := range []struct {
		catalog    *EnvironmentJSONStore
		domain, id string
	}{
		{first, "workspace", "dev"}, {first, "environment", "other"},
		{NewEnvironmentJSONStore(filepath.Join(filepath.Dir(path), "other.json")), "environment", "dev"},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		release, err := candidate.catalog.LockLifecycle(ctx, candidate.domain, candidate.id)
		cancel()
		if err != nil {
			t.Fatal("independent lock blocked", err)
		}
		release()
	}
}

func TestLifecycleLockRejectsUnsafeObjects(t *testing.T) {
	for _, kind := range []string{"directory-symlink", "directory-mode", "file-symlink", "file-hardlink", "file-mode"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "state")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			catalog := NewEnvironmentJSONStore(filepath.Join(dir, "environments.json"))
			unlock, err := catalog.LockLifecycle(context.Background(), "workspace", "dev")
			if err != nil {
				t.Fatal(err)
			}
			unlock()
			files, _ := filepath.Glob(filepath.Join(dir, "lifecycle-locks", "lifecycle-*.lock"))
			if len(files) != 1 {
				t.Fatal(files)
			}
			switch kind {
			case "directory-symlink":
				if err := os.Rename(dir, dir+"-real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(dir+"-real", dir); err != nil {
					t.Fatal(err)
				}
			case "directory-mode":
				if err := os.Chmod(dir, 0o777); err != nil {
					t.Fatal(err)
				}
			case "file-symlink":
				if err := os.Rename(files[0], files[0]+"-real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(files[0]+"-real", files[0]); err != nil {
					t.Fatal(err)
				}
			case "file-hardlink":
				if err := os.Link(files[0], files[0]+"-link"); err != nil {
					t.Fatal(err)
				}
			case "file-mode":
				if err := os.Chmod(files[0], 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if release, err := catalog.LockLifecycle(context.Background(), "workspace", "dev"); err == nil {
				release()
				t.Fatal("unsafe lock accepted")
			}
		})
	}
}

func TestLifecycleLockChild(t *testing.T) {
	path := os.Getenv("HACO_LIFECYCLE_LOCK_CHILD")
	if path == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	release, err := NewEnvironmentJSONStore(path).LockLifecycle(ctx, "environment", "dev")
	if release != nil {
		release()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("child escaped parent's lock: %v", err)
	}
}

func TestLifecycleLockSerializesProcessesAcrossTemporaryDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "environments.json")
	unlock, err := NewEnvironmentJSONStore(path).LockLifecycle(context.Background(), "environment", "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLifecycleLockChild$")
	child.Env = append(os.Environ(), "HACO_LIFECYCLE_LOCK_CHILD="+path, "TMPDIR="+t.TempDir())
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, output)
	}
}
