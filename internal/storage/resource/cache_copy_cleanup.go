package persistentresource

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type trackedCacheCopyBackend interface {
	CopyTracked(context.Context, core.PersistentResource, core.PersistentResource, func(string) error, func() error) error
	WaitCopyStopped(context.Context, core.PersistentResource) error
}

type cacheCopyStore interface {
	RecordCacheCopyOperation(context.Context, core.PersistentResource, string) (core.PersistentResource, error)
	MarkCacheCopyCleanup(context.Context, core.PersistentResource) (core.PersistentResource, error)
	BeginStoppedCacheCopyDelete(context.Context, core.PersistentResource) (core.PersistentResource, error)
}

func (s *Service) cleanupCacheCopy(ctx context.Context, target core.PersistentResource) (core.PersistentResource, error) {
	backend, ok := s.Backend.(trackedCacheCopyBackend)
	if !ok {
		return target, core.ErrRecoveryRequired
	}
	store, ok := s.Store.(cacheCopyStore)
	if !ok {
		return target, core.ErrRecoveryRequired
	}
	fenced, err := store.MarkCacheCopyCleanup(ctx, target)
	if err != nil {
		return target, errors.Join(err, core.ErrRecoveryRequired)
	}
	target = fenced
	if target.State != "deleting" {
		if err := backend.WaitCopyStopped(ctx, target); err != nil {
			return target, errors.Join(err, core.ErrRecoveryRequired)
		}
		fenced, err = store.BeginStoppedCacheCopyDelete(ctx, target)
		if err != nil {
			return target, errors.Join(err, core.ErrRecoveryRequired)
		}
		target = fenced
	}
	if err := s.finishDelete(ctx, target); err != nil {
		return target, err
	}
	return core.PersistentResource{}, nil
}
