package localraw

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type detachFailRunner struct {
	backing string
}

func (r detachFailRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name == "losetup" && len(args) == 2 && args[0] == "-j" && args[1] == r.backing {
		return host.Result{Stdout: fmt.Sprintf("/dev/loop999: []: (%s)\n", r.backing)}, nil
	}
	if name == "losetup" && len(args) == 2 && args[0] == "-d" && args[1] == "/dev/loop999" {
		return host.Result{ExitCode: 1}, errors.New("forced detach failure")
	}
	return host.Result{}, nil
}

type recordingRunner struct{ calls int }

func (r *recordingRunner) Run(context.Context, string, ...string) (host.Result, error) {
	r.calls++
	return host.Result{}, nil
}

type loopDetachRunner struct {
	backing       string
	currentDevice string
	detached      []string
	discoveryErr  error
}

func (r *loopDetachRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "losetup" || len(args) != 2 {
		return host.Result{}, fmt.Errorf("unexpected command: %s %v", name, args)
	}
	switch args[0] {
	case "-j":
		if args[1] != r.backing {
			return host.Result{}, fmt.Errorf("unexpected backing path: %s", args[1])
		}
		if r.discoveryErr != nil {
			return host.Result{ExitCode: 1}, r.discoveryErr
		}
		if r.currentDevice == "" {
			return host.Result{}, nil
		}
		return host.Result{Stdout: fmt.Sprintf("%s: []: (%s)\n", r.currentDevice, r.backing)}, nil
	case "-d":
		r.detached = append(r.detached, args[1])
		return host.Result{}, nil
	default:
		return host.Result{}, fmt.Errorf("unexpected losetup arguments: %v", args)
	}
}

func TestDeletePreservesBackingFileWhenDetachFails(t *testing.T) {
	path := trustedRawPath(t)
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(detachFailRunner{backing: path})
	err := store.Delete(context.Background(), block.Handle{Path: path, Device: "/dev/loop999"})
	if err == nil {
		t.Fatal("Delete succeeded despite detach failure")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("backing file was removed after detach failure: %v", statErr)
	}
}

func TestDetachResolvesCurrentLoopFromBackingPathInsteadOfCachedDevice(t *testing.T) {
	path := trustedRawPath(t)
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &loopDetachRunner{backing: path, currentDevice: "/dev/loop8"}
	store := New(runner)

	if err := store.Detach(context.Background(), block.Handle{Path: path, Device: "/dev/loop3"}); err != nil {
		t.Fatalf("Detach failed: %v", err)
	}
	if len(runner.detached) != 1 || runner.detached[0] != "/dev/loop8" {
		t.Fatalf("detached devices = %v, want only current /dev/loop8", runner.detached)
	}
}

func TestDetachDoesNotUseStaleCachedDeviceWhenBackingIsNotAttached(t *testing.T) {
	path := trustedRawPath(t)
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &loopDetachRunner{backing: path}
	store := New(runner)

	if err := store.Detach(context.Background(), block.Handle{Path: path, Device: "/dev/loop3"}); err != nil {
		t.Fatalf("Detach failed: %v", err)
	}
	if len(runner.detached) != 0 {
		t.Fatalf("detached stale cached device: %v", runner.detached)
	}
}

func TestDetachFailsClosedWhenCurrentLoopCannotBeDiscovered(t *testing.T) {
	path := trustedRawPath(t)
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &loopDetachRunner{backing: path, discoveryErr: errors.New("forced discovery failure")}
	store := New(runner)

	if err := store.Detach(context.Background(), block.Handle{Path: path, Device: "/dev/loop3"}); err == nil {
		t.Fatal("Detach succeeded despite loop discovery failure")
	}
	if len(runner.detached) != 0 {
		t.Fatalf("detached cached device after discovery failure: %v", runner.detached)
	}
}

func TestRawOperationsRejectSymlinkBackingImageWithoutTouchingVictim(t *testing.T) {
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
			path := trustedRawPath(t)
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

func TestRawEnsureRejectsNonRegularBackingObjects(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		path := trustedRawPath(t)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := New(&recordingRunner{}).Ensure(context.Background(), block.Spec{ID: "demo", Path: path, SizeBytes: 1}); err == nil {
			t.Fatal("Ensure accepted directory backing object")
		}
	})
	t.Run("fifo", func(t *testing.T) {
		path := trustedRawPath(t)
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Skipf("mkfifo unavailable: %v", err)
		}
		if _, err := New(&recordingRunner{}).Ensure(context.Background(), block.Spec{ID: "demo", Path: path, SizeBytes: 1}); err == nil {
			t.Fatal("Ensure accepted FIFO backing object")
		}
	})
}

func trustedRawPath(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "storage")
	images := filepath.Join(root, "images")
	if err := os.MkdirAll(images, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(images, "storage.raw")
}
