package persistentresource

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
)

// Import uses ordinary create ownership and publication. Archive metadata cannot
// select the new resource ID, owner or native destination. Failure keeps the
// existing creating record for exact-owned cleanup; no import recovery catalog.
func (s *Service) Import(ctx context.Context, id, kind string, archive io.ReadSeeker) (core.PersistentResource, error) {
	backend, ok := s.Backend.(interface {
		Import(context.Context, core.PersistentResource, io.ReadSeeker) error
	})
	if !ok {
		return core.PersistentResource{}, core.ErrUnsupported
	}
	if archive == nil {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	importing := Service{Store: s.Store, Backend: importBackend{Backend: s.Backend, create: func(ctx context.Context, r core.PersistentResource) error { return backend.Import(ctx, r, archive) }}}
	return importing.Create(ctx, id, kind)
}

type importBackend struct {
	Backend
	create func(context.Context, core.PersistentResource) error
}

func (b importBackend) Create(ctx context.Context, r core.PersistentResource) error {
	return b.create(ctx, r)
}
