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

const DefaultSocketPath = "/run/hacocoon/client.sock"
const DefaultPrivilegedSocketPath = "/run/hacocoon/control.sock"

func SocketPath() string {
	if path := strings.TrimSpace(os.Getenv("HACO_CONTROL_SOCKET")); path != "" {
		return path
	}
	return DefaultSocketPath
}

// PrivilegedSocketPath is the Physical Host endpoint projected into trusted
// haco-host. Development/test overrides deliberately collapse the client and
// privileged endpoints back to one caller-owned socket.
func PrivilegedSocketPath() string {
	if path := strings.TrimSpace(os.Getenv("HACO_CONTROL_SOCKET")); path != "" {
		return path
	}
	return DefaultPrivilegedSocketPath
}

func UnixDialer(path string) Dialer {
	return func(ctx context.Context) (net.Conn, error) {
		if strings.TrimSpace(path) == "" {
			return nil, ErrInvalidArgument
		}
		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, "unix", path)
		if err != nil {
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
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on Hacocoon control socket %q: %w", path, err)
	}
	if err := os.Chmod(path, mode.Perm()); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("set control socket permissions: %w", err)
	}
	return &unlinkListener{Listener: listener, path: path}, nil
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
	once sync.Once
}

func (l *unlinkListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { _ = os.Remove(l.path) })
	return err
}
