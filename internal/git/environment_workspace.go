package gitrepo

import (
	"context"
	"errors"
	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
	"time"
)

// PrepareEnvironmentWorkspace fixes membership at creation. Later registration
// changes never repopulate these independent copies.
func (s *RepositoryService) PrepareEnvironmentWorkspace(ctx context.Context, name string) (core.Workspace, error) {
	sources, err := s.ListSources(ctx)
	if err != nil {
		return core.Workspace{}, err
	}
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		ids = append(ids, source.Source.ID)
	}
	var object Object
	switch len(ids) {
	case 0:
		object, err = s.CreateEmptyWorkspace(ctx, name)
	case 1:
		object, err = s.CopyWorkspace(ctx, name, ids[0])
	default:
		object, err = s.CopyWorkspaceSet(ctx, name, ids)
	}
	if err != nil {
		if object.ID != "" {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			err = errors.Join(err, s.cleanupNewWorkspace(cleanup, object))
		}
		return core.Workspace{}, err
	}
	return core.Workspace{ID: core.WorkspaceID("workspace:managed:" + object.Owner), Path: "managed:" + object.ID}, nil
}

func (s *RepositoryService) CreateEmptyWorkspace(ctx context.Context, name string) (Object, error) {
	if !gitadapter.ValidID(name) {
		return Object{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	// The existing offline Workspace representation has no Git routing.
	object, err := s.createPrepared(ctx, Object{Kind: "work", ID: name, Repository: "empty"}, func(ctx context.Context, object Object) error {
		return s.Backend.CreateVolume(ctx, object, nil)
	}, nil)
	s.mu.Unlock()
	if err != nil && object.ID != "" {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if cleanupErr := s.cleanupNewWorkspace(cleanup, object); cleanupErr != nil {
			return object, errors.Join(err, cleanupErr, core.ErrRecoveryRequired)
		}
		return Object{}, err
	}
	return object, err
}

// Only an unpublished object returned by this creation is eligible. Native
// deletion rechecks exact ownership and absence, including ambiguous creates.
func (s *RepositoryService) cleanupNewWorkspace(ctx context.Context, expected Object) error {
	backend, ok := s.Backend.(interface {
		DeleteWorkspaceVolume(context.Context, Object) error
	})
	if !ok {
		return core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readObject("work", expected.ID)
	if err != nil {
		return err
	}
	if current.Owner != expected.Owner || current.State == "ready" {
		return core.ErrCapabilityStale
	}
	for _, member := range current.Copies() {
		if err := backend.DeleteWorkspaceVolume(ctx, member); err != nil {
			return errors.Join(err, core.ErrRecoveryRequired)
		}
	}
	return s.removeNewWorkspaceRecord(current)
}
