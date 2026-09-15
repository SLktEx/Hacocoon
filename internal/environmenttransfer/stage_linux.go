//go:build linux

package environmenttransfer

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/staging"
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
	m.Workspaces = append([]Workspace(nil), m.Workspaces...)
	m.Data = append([]Data(nil), m.Data...)
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
	if !validLimit(limit) {
		return nil, ErrInvalidBundle
	}
	readonly, n, err := staging.Capture(ctx, root, limit+envelopeOverhead, produce)
	if err != nil {
		if errors.Is(err, staging.ErrInvalidInput) {
			return nil, errors.Join(ErrInvalidBundle, err)
		}
		return nil, err
	}
	defer func() {
		if result == nil {
			err = errors.Join(err, readonly.Close())
		}
	}()
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
