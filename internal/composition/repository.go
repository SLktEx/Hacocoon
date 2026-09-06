package composition

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type repositoryWorkspaceProvider struct{ repositories *gitrepo.RepositoryService }

func (p repositoryWorkspaceProvider) Resolve(ctx context.Context, request workspaceapp.WorkspaceRequest) (core.Workspace, error) {
	if strings.HasPrefix(request.Path, "managed:") {
		return p.repositories.Workspace(ctx, strings.TrimPrefix(request.Path, "managed:"))
	}
	return workspaceapp.NewExternalPathWorkspace().Resolve(ctx, request)
}
