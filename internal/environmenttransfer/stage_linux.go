//go:build linux

package environmenttransfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Staged owns an unnamed file. No writable handle or filesystem path is exposed.
// This excludes pathname replacement; it is not a seal against Host administrators
// who already have authority to inspect or reopen process descriptors.
type Staged struct {
	file     *os.File
	manifest Manifest
	size     int64
}

func (s *Staged) Manifest() Manifest {
	m := s.manifest
	m.Components = append([]Component(nil), m.Components...)
	return m
}
func (s *Staged) Reader() io.Reader {
	return &stagedReader{reader: io.NewSectionReader(s.file, 0, s.size)}
}
func (s *Staged) Close() error { return s.file.Close() }

type stagedReader struct{ reader io.Reader }

func (r *stagedReader) Read(p []byte) (int, error) { return r.reader.Read(p) }

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// Stage accepts a controller-configured private directory, never a guest path.
// The caller owns transport cancellation: a blocked source Read must be unblocked
// by its transport. The byte budget also bounds invalid pre-manifest input.
// No O_TMPFILE fallback, named output, chmod repair or process-crash replay exists.
func Stage(ctx context.Context, root string, src io.Reader, limit int64) (result *Staged, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if src == nil || !validLimit(limit) || len(root) > 4096 || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, ErrInvalidBundle
	}
	dir, err := unix.Openat2(unix.AT_FDCWD, root, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC, Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, err
	}
	defer unix.Close(dir)
	var info unix.Stat_t
	if err := unix.Fstat(dir, &info); err != nil {
		return nil, err
	}
	if info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 || info.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil, errors.New("transfer staging directory must be private and owned by the controller")
	}
	fd, err := unix.Openat(dir, ".", unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	writable := os.NewFile(uintptr(fd), "transfer staging")
	defer func() {
		if writable != nil {
			err = errors.Join(err, writable.Close())
		}
	}()
	maximum := limit + envelopeOverhead
	input := &contextReader{ctx, src}
	n, err := io.Copy(writable, io.LimitReader(input, maximum))
	if err != nil {
		return nil, err
	}
	// Probe separately: adding one to the maximum budget can overflow int64.
	if err := requireEOF(input); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Reopen only our live descriptor, then close the sole writable descriptor.
	// The file has never had a directory entry; no path can be swapped for it.
	readFD, err := unix.Open(fmt.Sprintf("/proc/self/fd/%d", fd), unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	readonly := os.NewFile(uintptr(readFD), "verified transfer")
	defer func() {
		if result == nil {
			err = errors.Join(err, readonly.Close())
		}
	}()
	var before, after unix.Stat_t
	if err := unix.Fstat(fd, &before); err != nil {
		return nil, err
	}
	if err := unix.Fstat(readFD, &after); err != nil {
		return nil, err
	}
	if before.Dev != after.Dev || before.Ino != after.Ino || after.Nlink != 0 || after.Mode&unix.S_IFMT != unix.S_IFREG || after.Size != n {
		return nil, ErrInvalidBundle
	}
	err = writable.Close()
	writable = nil
	if err != nil {
		return nil, err
	}
	m, err := Inspect(&contextReader{ctx, io.NewSectionReader(readonly, 0, n)}, limit)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Staged{file: readonly, manifest: m, size: n}, nil
}
