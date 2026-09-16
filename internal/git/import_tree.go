package gitrepo

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ImportWorkspaceTree uses client-supplied bytes but only current registered
// Host routing. The common import transition owns publication and cleanup.
func (s *RepositoryService) ImportWorkspaceTree(ctx context.Context, id, repository string, source io.Reader) (Object, error) {
	if !gitadapter.ValidID(id) || !gitadapter.ValidID(repository) || source == nil {
		return Object{}, core.ErrInvalidArgument
	}
	backend, ok := s.Backend.(interface {
		ImportWorkspaceTreeVolume(context.Context, Object, io.Reader) error
	})
	if !ok {
		return Object{}, core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, err := s.Get("repo", repository)
	if err != nil {
		return Object{}, err
	}
	if err = s.Backend.InspectVolume(ctx, repo); err != nil {
		return Object{}, err
	}
	object := Object{Kind: "work", ID: id, Repository: repository, Remote: repo.Remote}
	return s.importPreparedWorkspace(ctx, object, func(ctx context.Context, o Object) error { return backend.ImportWorkspaceTreeVolume(ctx, o, source) })
}
