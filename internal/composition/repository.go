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

func (p repositoryWorkspaceProvider) ListManagedWorkspaces(ctx context.Context) ([]workspaceapp.ManagedWorkspace, error) {
	objects, err := p.repositories.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	result := []workspaceapp.ManagedWorkspace{}
	for _, o := range objects {
		repos := []string{}
		for _, member := range o.Copies() {
			repos = append(repos, member.Repository)
		}
		result = append(result, workspaceapp.ManagedWorkspace{Name: o.ID, State: o.State, Repositories: repos, Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + o.Owner), Path: "managed:" + o.ID}})
	}
	return result, nil
}
func (p repositoryWorkspaceProvider) DeleteWorkspace(ctx context.Context, work core.Workspace) error {
	if !strings.HasPrefix(work.Path, "managed:") || !strings.HasPrefix(string(work.ID), "workspace:managed:") {
		return core.ErrInvalidArgument
	}
	return p.repositories.DeleteWorkspace(ctx, strings.TrimPrefix(work.Path, "managed:"), strings.TrimPrefix(string(work.ID), "workspace:managed:"))
}
