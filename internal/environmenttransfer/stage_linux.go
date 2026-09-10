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
	spans    []componentSpan
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
	if src == nil {
		return nil, ErrInvalidBundle
	}
	return stageProduced(ctx, root, limit, func(dst io.Writer) error {
		input := &contextReader{ctx, src}
		if _, err := io.Copy(dst, io.LimitReader(input, limit+envelopeOverhead)); err != nil {
			return err
		}
		return requireEOF(input)
	})
}

// stageProduced keeps synchronous producers private until their entire operation,
// including cleanup, succeeds. No goroutine or partially published pathname exists.
func stageProduced(ctx context.Context, root string, limit int64, produce func(io.Writer) error) (result *Staged, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if produce == nil || !validLimit(limit) || len(root) > 4096 || !filepath.IsAbs(root) || filepath.Clean(root) != root {
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
	output := &stagingWriter{ctx: ctx, dst: writable, remaining: limit + envelopeOverhead}
	if err := produce(output); err != nil {
		return nil, err
	}
	n := limit + envelopeOverhead - output.remaining
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
	var spans []componentSpan
	m, err := inspect(&contextReader{ctx, io.NewSectionReader(readonly, 0, n)}, limit, &spans)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Staged{file: readonly, manifest: m, size: n, spans: spans}, nil
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
		return 0, ErrInvalidBundle
	}
	n, err := w.dst.Write(p)
	w.remaining -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

// ComponentReader supplies one verified native archive to the Incus importer.
// No outer-envelope bytes, path, writable handle or source authority escape.
// Readers have independent cursors and remain valid only while Staged is open.
// Inner archive validation and fresh destination ownership are separate duties.
func (s *Staged) ComponentReader(role string) (io.ReadSeeker, error) {
	if s == nil || s.file == nil {
		return nil, ErrInvalidBundle
	}
	for _, span := range s.spans {
		if span.component.Role == role {
			return &componentReader{reader: io.NewSectionReader(s.file, span.offset, span.component.Bytes)}, nil
		}
	}
	return nil, ErrInvalidBundle
}

type componentReader struct{ reader *io.SectionReader }

func (r *componentReader) Read(p []byte) (int, error) { return r.reader.Read(p) }
func (r *componentReader) Seek(offset int64, whence int) (int64, error) {
	return r.reader.Seek(offset, whence)
}
