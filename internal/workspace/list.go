package workspace

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type environmentLister interface {
	ListEnvironments(context.Context) ([]core.Environment, error)
}

func (s *Service) List(ctx context.Context) ([]core.Environment, error) {
	if s == nil || s.store == nil {
		return nil, core.ErrInvalidArgument
	}
	store, ok := s.store.(environmentLister)
	if !ok {
		return nil, fmt.Errorf("environment store does not support listing: %w", core.ErrUnsupported)
	}
	return store.ListEnvironments(ctx)
}
