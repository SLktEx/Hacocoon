package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
)

type runtimeCreation func(context.Context, core.EnvironmentRuntimeSpec, func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)
type archiveRuntimeCreator interface {
	CreateEnvironmentFromArchive(context.Context, core.EnvironmentRuntimeSpec, io.ReadSeeker, string, int64, func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error)
}

// CreateFromArchive attaches caller-prepared data through normal lifecycle
// ownership. The source reader stays caller-owned; no snapshot catalog or Base
// is synthesized. The provider owns only its temporary transport image.
func (s *Service) CreateFromArchive(ctx context.Context, spec core.EnvironmentSpec, source io.ReadSeeker, privateRoot string, limit int64) (core.Environment, error) {
	provider, ok := s.runtime.(archiveRuntimeCreator)
	if !ok {
		return core.Environment{}, core.ErrUnsupported
	}
	if source == nil || limit <= 0 || spec.Base != "" || spec.TemporaryWorkspace != nil {
		return core.Environment{}, core.ErrInvalidArgument
	}
	spec.SkipDefaultResource = spec.PersistentResource == ""
	return s.create(ctx, spec, nil, func(ctx context.Context, runtimeSpec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
		return provider.CreateEnvironmentFromArchive(ctx, runtimeSpec, source, privateRoot, limit, record)
	})
}
