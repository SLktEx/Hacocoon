package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"io"
	"time"
)

const MethodManagedWorkspace = "workspace.manage"

type ManagedWorkspaceRequest struct {
	Operation string         `json:"operation"`
	Workspace core.Workspace `json:"workspace"`
}
type managedWorkspaceService interface {
	ListManagedWorkspaces(context.Context) ([]workspace.ManagedWorkspace, error)
	DeleteManagedWorkspace(context.Context, core.Workspace) error
}

func RegisterManagedWorkspaces(server *control.Server, service managedWorkspaceService) error {
	return server.Register(MethodManagedWorkspace, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req ManagedWorkspaceRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
			return nil, translateError(core.ErrInvalidArgument)
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		switch req.Operation {
		case "list":
			if req.Workspace != (core.Workspace{}) {
				return nil, translateError(core.ErrInvalidArgument)
			}
			all, err := service.ListManagedWorkspaces(ctx)
			return all, translateError(err)
		case "delete":
			if req.Workspace.ID == "" || req.Workspace.Path == "" {
				return nil, translateError(core.ErrInvalidArgument)
			}
			return nil, translateError(service.DeleteManagedWorkspace(ctx, req.Workspace))
		default:
			return nil, translateError(core.ErrInvalidArgument)
		}
	})
}
func (c *Client) ListManagedWorkspaces(ctx context.Context) ([]workspace.ManagedWorkspace, error) {
	var result []workspace.ManagedWorkspace
	err := c.wire.Call(ctx, MethodManagedWorkspace, ManagedWorkspaceRequest{Operation: "list"}, &result)
	return result, err
}
func (c *Client) DeleteManagedWorkspace(ctx context.Context, work core.Workspace) error {
	return c.wire.Call(ctx, MethodManagedWorkspace, ManagedWorkspaceRequest{Operation: "delete", Workspace: work}, nil)
}
