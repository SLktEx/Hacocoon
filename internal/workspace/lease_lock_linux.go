//go:build linux

package workspace

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func lockWorkspace(ctx context.Context, id core.WorkspaceID) (func(), error) {
	dir := filepath.Join(os.TempDir(), "hacocoon-workspace-locks")
	if err := ensureTrustedWorkspaceLockDirectory(dir); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(id))
	path := filepath.Join(dir, fmt.Sprintf("%x.lock", sum))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open workspace lock: %w", err)
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = file.Close()
			return nil, fmt.Errorf("lock workspace: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func ensureTrustedWorkspaceLockDirectory(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create workspace lock directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("inspect workspace lock directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("workspace lock path %q is not a trusted directory", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("workspace lock directory %q is not owned by effective uid %d", dir, os.Geteuid())
	}
	if info.Mode().Perm() != 0o700 {
		return fmt.Errorf("workspace lock directory %q has unsafe permissions %o", dir, info.Mode().Perm())
	}
	return nil
}
