//go:build unix

package control

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// File-backed fixtures exercise the real pathname identity/removal code without
// pretending to provide the native UnixListener coverage below.
func TestUnlinkListenerCleanupOwnership(t *testing.T) {
	for _, kind := range []string{"owned", "missing", "file", "symlink", "directory", "unknown", "uninspectable"} {
		t.Run(kind, func(t *testing.T) {
			listener, original := fileBackedUnlinkListener(t)
			var replacement fs.FileInfo
			switch kind {
			case "missing", "file", "symlink", "directory":
				replacement = replaceCleanupPath(t, listener.path, kind)
			case "unknown":
				listener.info = nil
				replacement = original
			case "uninspectable":
				// A regular parent deterministically makes Lstat fail with ENOTDIR.
				listener.path = filepath.Join(listener.path, "child")
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			if replacement != nil {
				assertCleanupIdentity(t, listener.path, replacement)
			} else if kind == "uninspectable" {
				assertCleanupIdentity(t, filepath.Dir(listener.path), original)
			} else if _, err := os.Lstat(listener.path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("cleanup path still exists or cannot be inspected: %v", err)
			}
		})
	}
}

func TestUnlinkListenerReplacementDuringClose(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			listener, _ := fileBackedUnlinkListener(t)
			var replacement fs.FileInfo
			closeErr := errors.New("listener close failed")
			listener.Listener = cleanupTestListener{close: func() error {
				replacement = replaceCleanupPath(t, listener.path, kind)
				return closeErr
			}}
			if err := listener.Close(); !errors.Is(err, closeErr) {
				t.Fatalf("close error = %v", err)
			}
			assertCleanupIdentity(t, listener.path, replacement)
		})
	}
}

func TestUnlinkListenerRepeatedAndConcurrentClose(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "replacement"}[replace], func(t *testing.T) {
			checkUnlinkConcurrentClose(t, replace)
		})
	}
}

func checkUnlinkConcurrentClose(t *testing.T, replace bool) {
	t.Helper()
	listener, _ := fileBackedUnlinkListener(t)
	var replacement fs.FileInfo
	if replace {
		replacement = replaceCleanupPath(t, listener.path, "file")
	}
	closeErr := errors.New("listener close result")
	var calls atomic.Int32
	listener.Listener = cleanupTestListener{close: func() error {
		calls.Add(1)
		return closeErr
	}}
	const count = 16
	if successful := closeCleanupConcurrently(t, listener, closeErr, count); successful != 0 {
		t.Fatalf("successful closes = %d, want 0", successful)
	}
	if calls.Load() != count {
		t.Fatalf("underlying close calls = %d, want %d", calls.Load(), count)
	}
	if replace {
		assertCleanupIdentity(t, listener.path, replacement)
	} else {
		if _, err := os.Lstat(listener.path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owned path was not removed: %v", err)
		}
		writeCleanupFile(t, listener.path)
		replacement = cleanupPathInfo(t, listener.path)
	}
	if err := listener.Close(); !errors.Is(err, closeErr) {
		t.Fatal(err)
	}
	assertCleanupIdentity(t, listener.path, replacement)
}

func TestCaptureUnixListenerFailurePreservesPath(t *testing.T) {
	for _, kind := range []string{"missing", "uninspectable", "file", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			checkCaptureUnixListenerFailure(t, kind)
		})
	}
}

func checkCaptureUnixListenerFailure(t *testing.T, kind string) {
	t.Helper()
	path, retained, wantErr := captureFailurePath(t, kind)
	var closed bool
	var disabled bool
	var replacement fs.FileInfo
	if retained != "" {
		replacement = cleanupPathInfo(t, retained)
	}
	listener := cleanupUnixTestListener{
		cleanupTestListener: cleanupTestListener{close: func() error {
			closed = true
			if !disabled {
				t.Fatal("identity failure closed listener before disabling automatic unlink")
			}
			if kind == "missing" {
				writeCleanupFile(t, path)
				retained = path
				replacement = cleanupPathInfo(t, path)
			}
			return errors.New("underlying close failure")
		}},
		setUnlink: func(unlink bool) { disabled = !unlink },
	}
	owned, err := captureUnixListener(listener, path)
	if owned != nil || !errors.Is(err, wantErr) || !closed || !disabled {
		t.Fatalf("capture failure = %v, %v; closed=%v disabled=%v", owned, err, closed, disabled)
	}
	assertCleanupIdentity(t, retained, replacement)
}

func captureFailurePath(t *testing.T, kind string) (path, retained string, wantErr error) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "control.sock")
	switch kind {
	case "missing":
		return path, "", os.ErrNotExist
	case "uninspectable":
		writeCleanupFile(t, path)
		return filepath.Join(path, "child"), path, syscall.ENOTDIR
	case "file":
		writeCleanupFile(t, path)
	case "symlink":
		writeCleanupFile(t, path+".target")
		if err := os.Symlink(path+".target", path); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown capture failure kind %q", kind)
	}
	return path, path, ErrAlreadyRunning
}

type cleanupUnixTestListener struct {
	cleanupTestListener
	setUnlink func(bool)
}

func (l cleanupUnixTestListener) SetUnlinkOnClose(unlink bool) { l.setUnlink(unlink) }

func TestFinishUnixListenerPermissionFailure(t *testing.T) {
	for _, kind := range []string{"owned", "missing", "file", "symlink", "directory", "during-close"} {
		t.Run(kind, func(t *testing.T) {
			checkFinishPermissionFailure(t, kind)
		})
	}
}

func checkFinishPermissionFailure(t *testing.T, kind string) {
	t.Helper()
	listener, original := fileBackedUnlinkListener(t)
	chmodErr := errors.New("chmod failed")
	closeErr := errors.New("close failed")
	var closed bool
	var replacement fs.FileInfo
	listener.Listener = cleanupTestListener{close: func() error {
		closed = true
		if kind == "during-close" {
			replacement = replaceCleanupPath(t, listener.path, "file")
		}
		return closeErr
	}}
	got, err := finishUnixListener(listener, 0660|fs.ModeSetuid, func(path string, mode fs.FileMode) error {
		if path != listener.path || mode != 0660 {
			t.Fatalf("chmod arguments = %q, %o", path, mode)
		}
		if kind != "owned" && kind != "during-close" {
			replacement = replaceCleanupPath(t, path, kind)
		}
		return chmodErr
	})
	if got != nil || !errors.Is(err, chmodErr) || errors.Is(err, closeErr) || !closed {
		t.Fatalf("permission failure = %v, %v; closed = %v", got, err, closed)
	}
	if replacement != nil {
		assertCleanupIdentity(t, listener.path, replacement)
	} else if _, err := os.Lstat(listener.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned/missing path not absent: %v", err)
	}
	if kind != "owned" {
		assertCleanupIdentity(t, listener.path+".original", original)
	}
}

func TestFinishUnixListenerPermissions(t *testing.T) {
	listener, original := fileBackedUnlinkListener(t)
	defer func() { _ = listener.Close() }()
	got, err := finishUnixListener(listener, 0660|fs.ModeSetuid, os.Chmod)
	if err != nil || got != listener {
		t.Fatalf("finish = %v, %v", got, err)
	}
	if !os.SameFile(original, cleanupPathInfo(t, listener.path)) {
		t.Fatal("permission setup changed the owned identity")
	}
	if mode := cleanupPathInfo(t, listener.path).Mode().Perm(); mode != 0660 {
		t.Fatalf("mode = %o, want 660", mode)
	}
}

func TestListenUnixCleanupOwnership(t *testing.T) {
	for _, kind := range []string{"owned", "missing", "file", "symlink", "directory", "socket"} {
		t.Run(kind, func(t *testing.T) {
			checkListenCleanupOwnership(t, kind)
		})
	}
}

func checkListenCleanupOwnership(t *testing.T, kind string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	var replacement fs.FileInfo
	if kind != "owned" {
		replacement = replaceCleanupPath(t, path, kind)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if replacement != nil {
		assertCleanupIdentity(t, path, replacement)
	} else if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned/missing socket was not removed: %v", err)
	}
	if err := listener.Close(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("repeat close = %v, want net.ErrClosed", err)
	}
	if replacement != nil {
		assertCleanupIdentity(t, path, replacement)
	}
	if kind == "socket" {
		conn, err := net.DialTimeout("unix", path, time.Second)
		if err != nil {
			t.Fatalf("replacement listener became unreachable: %v", err)
		}
		_ = conn.Close()
	}
}

func TestListenUnixDisablesAutomaticUnlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	replacement := replaceCleanupPath(t, path, "file")
	// Check Go's underlying Close independently of the wrapper's own cleanup.
	if err := listener.(*unlinkListener).Listener.Close(); err != nil {
		t.Fatal(err)
	}
	assertCleanupIdentity(t, path, replacement)
	if err := listener.Close(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("wrapper close = %v, want net.ErrClosed", err)
	}
	assertCleanupIdentity(t, path, replacement)
}

func TestListenUnixPermissionFailureCleanup(t *testing.T) {
	for _, kind := range []string{"owned", "socket"} {
		t.Run(kind, func(t *testing.T) {
			checkListenPermissionFailure(t, kind)
		})
	}
}

func checkListenPermissionFailure(t *testing.T, kind string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	chmodErr := errors.New("chmod failed")
	var replacement fs.FileInfo
	got, err := finishUnixListener(listener.(*unlinkListener), 0600, func(string, fs.FileMode) error {
		if kind == "socket" {
			replacement = replaceCleanupPath(t, path, kind)
		}
		return chmodErr
	})
	if got != nil || !errors.Is(err, chmodErr) {
		t.Fatalf("permission failure = %v, %v", got, err)
	}
	if replacement != nil {
		assertCleanupIdentity(t, path, replacement)
	} else if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned socket not absent: %v", err)
	}
}

func TestListenUnixConcurrentClose(t *testing.T) {
	for _, kind := range []string{"owned", "socket"} {
		t.Run(kind, func(t *testing.T) {
			checkListenConcurrentClose(t, kind)
		})
	}
}

func checkListenConcurrentClose(t *testing.T, kind string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	var replacement fs.FileInfo
	if kind == "socket" {
		replacement = replaceCleanupPath(t, path, kind)
	}
	if successful := closeCleanupConcurrently(t, listener, net.ErrClosed, 16); successful != 1 {
		t.Fatalf("successful closes = %d, want 1", successful)
	}
	if replacement != nil {
		assertCleanupIdentity(t, path, replacement)
	} else if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned socket not absent: %v", err)
	}
}

func TestListenUnixRequestedPermissions(t *testing.T) {
	for _, mode := range []fs.FileMode{0, 0600, 0660, 0660 | fs.ModeSetuid} {
		path := filepath.Join(t.TempDir(), "control.sock")
		listener, err := ListenUnix(path, mode)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		want := mode.Perm()
		if mode == 0 {
			want = 0600
		}
		if got := cleanupPathInfo(t, path).Mode().Perm(); got != want {
			t.Errorf("mode = %o, want %o", got, want)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

type cleanupTestListener struct {
	net.Listener
	close func() error
}

func (l cleanupTestListener) Close() error { return l.close() }

func fileBackedUnlinkListener(t *testing.T) (*unlinkListener, fs.FileInfo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	writeCleanupFile(t, path)
	info := cleanupPathInfo(t, path)
	return &unlinkListener{Listener: cleanupTestListener{close: func() error { return nil }}, path: path, info: info}, info
}

func replaceCleanupPath(t *testing.T, path, kind string) fs.FileInfo {
	t.Helper()
	// Retain the original inode so the replacement cannot reuse its identity.
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "missing":
		return nil
	case "file":
		writeCleanupFile(t, path)
	case "symlink":
		if err := os.Symlink(path+".original", path); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	case "socket":
		listener, err := ListenUnix(path, 0600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
	default:
		t.Fatalf("unknown replacement kind %q", kind)
	}
	return cleanupPathInfo(t, path)
}

func writeCleanupFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test-owned data"), 0600); err != nil {
		t.Fatal(err)
	}
}

func cleanupPathInfo(t *testing.T, path string) fs.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func assertCleanupIdentity(t *testing.T, path string, want fs.FileInfo) {
	t.Helper()
	got := cleanupPathInfo(t, path)
	if !os.SameFile(want, got) || want.Mode() != got.Mode() {
		t.Fatalf("replacement identity or mode changed at %q", path)
	}
}

func closeCleanupConcurrently(t *testing.T, listener net.Listener, wantErr error, count int) int32 {
	t.Helper()
	var successful atomic.Int32
	var group sync.WaitGroup
	for range count {
		group.Go(func() {
			if err := listener.Close(); err == nil {
				successful.Add(1)
			} else if !errors.Is(err, wantErr) {
				t.Errorf("unexpected close error: %v", err)
			}
		})
	}
	group.Wait()
	return successful.Load()
}
