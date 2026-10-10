package control

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const DefaultSocketPath = "/run/hacocoon/control.sock"

func SocketPath() string {
	if path := strings.TrimSpace(os.Getenv("HACO_CONTROL_SOCKET")); path != "" {
		return path
	}
	return DefaultSocketPath
}

func UnixDialer(path string) Dialer {
	return func(ctx context.Context) (net.Conn, error) {
		if strings.TrimSpace(path) == "" {
			return nil, ErrInvalidArgument
		}
		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, "unix", path)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("dial Hacocoon control socket %q: %v: %w", path, err, ErrUnavailable)
		}
		return conn, nil
	}
}

func ListenUnix(path string, mode fs.FileMode) (net.Listener, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, ErrInvalidArgument
	}
	if mode == 0 {
		mode = 0o600
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create control socket directory: %w", err)
	}
	if err := removeStaleSocket(path); err != nil {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on Hacocoon control socket %q: %w", path, err)
	}
	owned, err := captureUnixListener(listener, path)
	if err != nil {
		return nil, err
	}
	return finishUnixListener(owned, mode, os.Chmod)
}

type unixSocketListener interface {
	net.Listener
	SetUnlinkOnClose(bool)
}

func captureUnixListener(listener unixSocketListener, path string) (*unlinkListener, error) {
	// Go's default Close unlinks by pathname, even if another object replaced it.
	// Disable that before any failure path can close the listener. Binding and
	// capturing this identity require the parent to remain under trusted control.
	listener.SetUnlinkOnClose(false)
	info, err := os.Lstat(path)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("inspect newly created control socket %q: %w", path, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		_ = listener.Close()
		return nil, fmt.Errorf("new control socket path %q is not a socket: %w", path, ErrAlreadyRunning)
	}
	return &unlinkListener{Listener: listener, path: path, info: info}, nil
}

func finishUnixListener(listener *unlinkListener, mode fs.FileMode, chmod func(string, fs.FileMode) error) (net.Listener, error) {
	if err := chmod(listener.path, mode.Perm()); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("set control socket permissions: %w", err)
	}
	return listener, nil
}

func removeStaleSocket(path string) error {
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect control socket %q: %w", path, err)
	}
	if before.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("control socket path %q exists and is not a socket: %w", path, ErrAlreadyRunning)
	}

	conn, dialErr := net.DialTimeout("unix", path, 50*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		return fmt.Errorf("control socket %q is active: %w", path, ErrAlreadyRunning)
	}
	if errors.Is(dialErr, os.ErrNotExist) {
		return nil
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return fmt.Errorf("cannot prove control socket %q is stale: %v: %w", path, dialErr, ErrAlreadyRunning)
	}

	after, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("recheck control socket %q: %w", path, err)
	}
	if after.Mode()&os.ModeSocket == 0 || !os.SameFile(before, after) {
		return fmt.Errorf("control socket path %q changed during stale check: %w", path, ErrAlreadyRunning)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale control socket %q: %w", path, err)
	}
	return nil
}

type unlinkListener struct {
	net.Listener
	path string
	info fs.FileInfo
	once sync.Once
}

func (l *unlinkListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() {
		current, statErr := os.Lstat(l.path)
		if statErr != nil || l.info == nil || current.Mode().Type() != l.info.Mode().Type() || !os.SameFile(l.info, current) {
			return
		}
		// This protects stable replacements, not hostile concurrent directory
		// writers: Lstat and Remove are not atomic. The endpoint's parent must
		// remain under trusted control, as for bind and permission setup.
		_ = os.Remove(l.path)
	})
	return err
}
