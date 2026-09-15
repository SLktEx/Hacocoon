//go:build linux

package controller

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

func registerWorkspaceImport(server *control.Server, app *composition.App) error {
	root := os.Getenv("HACO_ROOT")
	if root == "" {
		root = "/var/lib/hacocoon"
	}
	return controlapi.RegisterWorkspaceImport(server, func(ctx context.Context, r io.Reader, req controlapi.WorkspaceImportRequest) (controlapi.WorkspaceImportResult, error) {
		result := controlapi.WorkspaceImportResult{Repository: req.Repository}
		if err := os.MkdirAll(filepath.Join(root, "transfers"), 0700); err != nil {
			return result, err
		}
		object, err := app.Repositories.ImportWorkspaceTree(ctx, req.Name, req.Repository, r)
		if object.ID != "" {
			result.Reference = workflow.Reference{Name: object.ID, Workspace: core.WorkspaceID("workspace:managed:" + object.Owner)}
			result.State = "recovery-required"
		}
		if err == nil {
			result.State = "ready"
		}
		return result, err
	})
}
