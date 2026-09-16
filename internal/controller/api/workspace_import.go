package controlapi

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

const MethodWorkspaceImport = "workspace.import"

type WorkspaceImportRequest struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
}
type WorkspaceImportResult struct {
	workflow.Reference
	Repository string `json:"repository"`
	State      string `json:"state"`
}

func (r WorkspaceImportRequest) Validate() error {
	if !gitadapter.ValidID(r.Name) || !gitadapter.ValidID(r.Repository) {
		return core.ErrInvalidArgument
	}
	return nil
}
func validWorkspaceImportResult(r WorkspaceImportResult) bool {
	if r.Name != "" && !gitadapter.ValidID(r.Name) || r.Repository != "" && !gitadapter.ValidID(r.Repository) {
		return false
	}
	if r.Workspace != "" && (!strings.HasPrefix(string(r.Workspace), "workspace:managed:") || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: strings.TrimPrefix(string(r.Workspace), "workspace:managed:")})) {
		return false
	}
	return r.State == "" || r.State == "ready" || r.State == "recovery-required"
}
func RegisterWorkspaceImport(server *control.Server, receive func(context.Context, io.Reader, WorkspaceImportRequest) (WorkspaceImportResult, error)) error {
	if receive == nil {
		return core.ErrInvalidArgument
	}
	return registerImportStream(server, MethodWorkspaceImport, func(payload json.RawMessage) (func(context.Context, io.Reader) (WorkspaceImportResult, error), error) {
		var req WorkspaceImportRequest
		if decodeExportJSON(payload, &req) != nil || req.Validate() != nil {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, r io.Reader) (WorkspaceImportResult, error) { return receive(ctx, r, req) }, nil
	}, func(r WorkspaceImportResult) bool { return r.State == "ready" })
}
func (c *Client) ImportWorkspace(ctx context.Context, source io.Reader, req WorkspaceImportRequest) (WorkspaceImportResult, error) {
	if source == nil || req.Validate() != nil {
		return WorkspaceImportResult{}, core.ErrInvalidArgument
	}
	return uploadInput(ctx, c, source, MethodWorkspaceImport, req, validWorkspaceImportResult, func(r WorkspaceImportResult) bool {
		return r.State == "ready" && r.Name == req.Name && r.Repository == req.Repository && r.Workspace != ""
	})
}
