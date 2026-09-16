//go:build linux

package basebuild

import (
	"context"
	"errors"
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/staging"
)

type archiveEnvironments interface {
	CreateFromArchive(context.Context, core.EnvironmentSpec, io.ReadSeeker, string, int64) (core.Environment, error)
}

// Import captures the complete bounded stream before creating a temporary Env.
// Native image validation/temporary image ownership remain in the provider.
func (s *Service) Import(ctx context.Context, req ImportRequest, source io.Reader, root string) (result Result, err error) {
	if s == nil || s.Environments == nil {
		return result, core.ErrUnsupported
	}
	if req.Validate() != nil || source == nil {
		return result, core.ErrInvalidArgument
	}
	creator, ok := s.Environments.(archiveEnvironments)
	if !ok {
		return result, core.ErrUnsupported
	}
	input, n, err := staging.Capture(ctx, root, MaxArchiveBytes, func(dst io.Writer) error {
		_, err := io.Copy(dst, io.LimitReader(&importReader{ctx, source}, MaxArchiveBytes+1))
		return err
	})
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	if n == 0 {
		return result, core.ErrInvalidArgument
	}
	return s.build(ctx, req.Name, "", func(ctx context.Context, name string, work core.Workspace) (core.Environment, error) {
		return creator.CreateFromArchive(ctx, core.EnvironmentSpec{Name: name, TemporaryWorkspace: &work, SkipDefaultResource: true, Resources: builderResources()}, io.NewSectionReader(input, 0, n), root, MaxArchiveBytes)
	}, nil)
}

type importReader struct {
	ctx    context.Context
	source io.Reader
}

func (r *importReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}
