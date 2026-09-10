package gitrepo

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"strings"
)

// ImportWorkspace registers one independent archive without cloning, checking out
// or executing guest Git configuration. The public aggregate importer is separate.
// Local source remotes and absent routing require an explicit offline/rebinding
// policy; this initial registration primitive does not adopt destination Host paths.
func (s *RepositoryService) ImportWorkspace(ctx context.Context, id, repository, remote, branch string, archive io.ReadSeeker) (Object, error) {
	if !ValidID(id) || !ValidID(repository) || !ValidBranch(branch) || ValidateRemote(remote) != nil || archive == nil {
		return Object{}, core.ErrInvalidArgument
	}
	if !strings.HasPrefix(remote, "https://github.com/") {
		return Object{}, core.ErrUnsupported
	}
	backend, ok := s.Backend.(interface {
		ImportWorkspaceVolume(context.Context, Object, io.ReadSeeker) error
	})
	if !ok {
		return Object{}, core.ErrUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	object := Object{Kind: "work", ID: id, Repository: repository, Remote: remote, Branch: branch}
	return s.createPrepared(ctx, object, func(ctx context.Context, o Object) error { return backend.ImportWorkspaceVolume(ctx, o, archive) }, nil)
}
