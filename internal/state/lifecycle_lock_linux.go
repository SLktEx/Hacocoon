//go:build linux

package state

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// LockLifecycle serializes a complete provider operation within this catalog.
// It is distinct from the short catalog transaction lock and must precede it.
func (s *EnvironmentJSONStore) LockLifecycle(ctx context.Context, domain, id string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if (domain != "environment" && domain != "workspace") || id == "" {
		return nil, fmt.Errorf("invalid lifecycle lock identity")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create lifecycle state directory: %w", err)
	}
	directory, err := unix.Openat2(unix.AT_FDCWD, dir, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("open lifecycle state directory: %w", err)
	}
	defer func() { _ = unix.Close(directory) }()
	var parent unix.Stat_t
	if err := unix.Fstat(directory, &parent); err != nil {
		return nil, fmt.Errorf("inspect lifecycle state directory: %w", err)
	}
	if parent.Uid != uint32(os.Geteuid()) || parent.Mode&unix.S_IFMT != unix.S_IFDIR || parent.Mode&0o022 != 0 {
		return nil, fmt.Errorf("unsafe lifecycle state directory ownership or permissions")
	}
	if err := unix.Mkdirat(directory, "lifecycle-locks", 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
		return nil, fmt.Errorf("create lifecycle lock directory: %w", err)
	}
	locks, err := unix.Openat(directory, "lifecycle-locks", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open lifecycle lock directory: %w", err)
	}
	defer func() { _ = unix.Close(locks) }()
	if err := validateLifecycleLock(locks, true); err != nil {
		return nil, err
	}
	// Include the catalog filename: independent catalogs may share a directory.
	sum := sha256.Sum256([]byte(filepath.Base(s.path) + "\x00" + domain + "\x00" + id))
	name := fmt.Sprintf("lifecycle-%x.lock", sum)
	fd, err := unix.Openat(locks, name, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lifecycle lock: %w", err)
	}
	if err := validateLifecycleLock(fd, false); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = unix.Close(fd)
			return nil, err
		}
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() {
				_ = unix.Flock(fd, unix.LOCK_UN)
				_ = unix.Close(fd)
			}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			_ = unix.Close(fd)
			return nil, fmt.Errorf("lock lifecycle: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = unix.Close(fd)
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func validateLifecycleLock(fd int, directory bool) error {
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return fmt.Errorf("inspect lifecycle lock: %w", err)
	}
	want := uint32(unix.S_IFREG | 0o600)
	if directory {
		want = unix.S_IFDIR | 0o700
	}
	if info.Uid != uint32(os.Geteuid()) || info.Mode != want || (!directory && info.Nlink != 1) {
		return fmt.Errorf("unsafe lifecycle lock ownership, type or permissions")
	}
	return nil
}
