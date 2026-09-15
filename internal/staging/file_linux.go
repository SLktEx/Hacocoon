//go:build linux

// Package staging captures bounded uploads in controller-owned unnamed files.
package staging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

var ErrInvalidInput = errors.New("invalid or oversized staged input")

// Capture returns only an unnamed read-only descriptor after the producer has
// completed. The controller configures root; neither guest nor client chooses it.
// The caller owns cancellation of blocked source reads and closes the result.
func Capture(ctx context.Context, root string, limit int64, produce func(io.Writer) error) (result *os.File, size int64, err error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if produce == nil || limit <= 0 || len(root) > 4096 || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, 0, ErrInvalidInput
	}
	dir, err := unix.Openat2(unix.AT_FDCWD, root, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC, Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = unix.Close(dir) }()
	var info unix.Stat_t
	if err := unix.Fstat(dir, &info); err != nil {
		return nil, 0, err
	}
	if info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 || info.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil, 0, errors.New("transfer staging directory must be private and owned by the controller")
	}
	fd, err := unix.Openat(dir, ".", unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, 0, err
	}
	writable := os.NewFile(uintptr(fd), "transfer staging")
	defer func() {
		if writable != nil {
			err = errors.Join(err, writable.Close())
		}
	}()
	output := &stagingWriter{ctx: ctx, dst: writable, remaining: limit}
	if err := produce(output); err != nil {
		return nil, 0, err
	}
	n := limit - output.remaining
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	// Reopen only our live descriptor, then close the sole writable descriptor.
	// The file has never had a directory entry; no path can be swapped for it.
	readFD, err := unix.Open(fmt.Sprintf("/proc/self/fd/%d", fd), unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, 0, err
	}
	readonly := os.NewFile(uintptr(readFD), "verified transfer")
	defer func() {
		if result == nil {
			err = errors.Join(err, readonly.Close())
		}
	}()
	var before, after unix.Stat_t
	if err := unix.Fstat(fd, &before); err != nil {
		return nil, 0, err
	}
	if err := unix.Fstat(readFD, &after); err != nil {
		return nil, 0, err
	}
	if before.Dev != after.Dev || before.Ino != after.Ino || after.Nlink != 0 || after.Mode&unix.S_IFMT != unix.S_IFREG || after.Size != n {
		return nil, 0, ErrInvalidInput
	}
	err = writable.Close()
	writable = nil
	if err != nil {
		return nil, 0, err
	}
	return readonly, n, nil
}

// Never expose the staging file itself to a producer. Bound even faulty writers.
type stagingWriter struct {
	ctx       context.Context
	dst       io.Writer
	remaining int64
}

func (w *stagingWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, ErrInvalidInput
	}
	n, err := w.dst.Write(p)
	w.remaining -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
