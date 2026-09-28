//go:build linux

package basebuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"runtime"

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
	input, n, err := staging.Capture(ctx, root, req.ArchiveLimit(), func(dst io.Writer) error {
		_, err := io.Copy(dst, io.LimitReader(&importReader{ctx, source}, req.ArchiveLimit()+1))
		return err
	})
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	builder := ""
	if a := req.Artifact; a != nil {
		if n != a.Size {
			return result, core.ErrInvalidArgument
		}
		digest := sha256.New()
		if _, err := io.Copy(digest, &importReader{ctx, io.NewSectionReader(input, 0, n)}); err != nil {
			return result, err
		}
		if hex.EncodeToString(digest.Sum(nil)) != a.SHA256 {
			return result, core.ErrInvalidArgument
		}
		arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
		if arch == "" || a.Architecture != arch {
			return result, core.ErrUnsupported
		}
		builder = "build-" + a.ID
	}
	if n == 0 {
		return result, core.ErrInvalidArgument
	}
	return s.build(ctx, req.Name, builder, func(ctx context.Context, name string, work core.Workspace) (core.Environment, error) {
		return creator.CreateFromArchive(ctx, core.EnvironmentSpec{Name: name, TemporaryWorkspace: &work, SkipDefaultResource: true, Resources: importBuilderResources(n)}, io.NewSectionReader(input, 0, n), root, req.ArchiveLimit())
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

func importBuilderResources(size int64) core.ResourceBudget {
	budget := builderResources()
	if uint64(size) > core.MaxRootDiskResourceBytes/2 {
		// Core cannot represent a finite quota this large. Preserve the other
		// resource budgets and use its explicit unlimited disk mode.
		budget.RootBytes = core.ResourceLimit{Mode: core.ResourceLimitUnlimited}
	} else if uint64(size)*2 > budget.RootBytes.Value {
		budget.RootBytes.Value = uint64(size) * 2
	}
	return budget
}
