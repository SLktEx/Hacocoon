package localqcow2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type detachFailRunner struct{}

func (detachFailRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name == "qemu-nbd" && len(args) == 2 && args[0] == "--disconnect" {
		return host.Result{ExitCode: 1}, errors.New("forced detach failure")
	}
	return host.Result{}, nil
}

type recordingRunner struct{ calls int }

func (r *recordingRunner) Run(context.Context, string, ...string) (host.Result, error) {
	r.calls++
	return host.Result{}, nil
}

func TestDeletePreservesBackingFileWhenDetachFails(t *testing.T) {
	path := trustedQCOW2Path(t)
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(detachFailRunner{})
	err := store.Delete(context.Background(), block.Handle{Path: path, Device: "/dev/nbd999"})
	if err == nil {
		t.Fatal("Delete succeeded despite detach failure")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("backing file was removed after detach failure: %v", statErr)
	}
}

func TestQCOW2OperationsRejectSymlinkBackingImageWithoutTouchingVictim(t *testing.T) {
	operations := map[string]func(*Store, string) error{
		"ensure": func(store *Store, path string) error {
			_, err := store.Ensure(context.Background(), block.Spec{ID: "demo", Path: path, SizeBytes: 1})
			return err
		},
		"grow": func(store *Store, path string) error {
			_, err := store.Grow(context.Background(), block.Handle{ID: "demo", Path: path}, 1)
			return err
		},
		"shrink": func(store *Store, path string) error {
			_, err := store.Shrink(context.Background(), block.Handle{ID: "demo", Path: path}, 1)
			return err
		},
		"delete": func(store *Store, path string) error {
			return store.Delete(context.Background(), block.Handle{ID: "demo", Path: path})
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			path := trustedQCOW2Path(t)
			victim := filepath.Join(filepath.Dir(filepath.Dir(path)), "victim")
			want := []byte("sentinel-do-not-touch")
			if err := os.WriteFile(victim, want, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(victim, path); err != nil {
				t.Fatal(err)
			}
			runner := &recordingRunner{}
			if err := operation(New(runner), path); err == nil {
				t.Fatalf("%s accepted symlink backing image", name)
			}
			got, err := os.ReadFile(victim)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("victim changed: got %q want %q", got, want)
			}
			if runner.calls != 0 {
				t.Fatalf("%s invoked privileged runner after path rejection: %d calls", name, runner.calls)
			}
		})
	}
}

func TestQCOW2EnsureRejectsNonRegularBackingObjects(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		path := trustedQCOW2Path(t)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := New(&recordingRunner{}).Ensure(context.Background(), block.Spec{ID: "demo", Path: path, SizeBytes: 1}); err == nil {
			t.Fatal("Ensure accepted directory backing object")
		}
	})
	t.Run("fifo", func(t *testing.T) {
		path := trustedQCOW2Path(t)
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Skipf("mkfifo unavailable: %v", err)
		}
		if _, err := New(&recordingRunner{}).Ensure(context.Background(), block.Spec{ID: "demo", Path: path, SizeBytes: 1}); err == nil {
			t.Fatal("Ensure accepted FIFO backing object")
		}
	})
}

func trustedQCOW2Path(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "storage")
	images := filepath.Join(root, "images")
	if err := os.MkdirAll(images, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(images, "storage.qcow2")
}
