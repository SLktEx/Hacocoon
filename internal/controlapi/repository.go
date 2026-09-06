package controlapi

import (
	"context"
	"encoding/json"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

const (
	MethodRepositoryClone = "repository.clone"
	MethodWorkspaceCopy   = "workspace.copy"
	MethodGitConnect      = "git.connect"
	MethodGitPending      = "git.pending"
	MethodGitDecide       = "git.decide"
)

type RepositoryCloneRequest struct {
	ID     string `json:"id"`
	Remote string `json:"remote"`
	Branch string `json:"branch"`
}
type WorkspaceCopyRequest struct {
	ID           string   `json:"id"`
	Repository   string   `json:"repository"`
	Repositories []string `json:"repositories,omitempty"`
}
type GitDecisionRequest struct {
	ID       string `json:"id"`
	Approved bool   `json:"approved"`
}

func RegisterRepositories(server *control.Server, repositories *gitrepo.RepositoryService, broker *gitrepo.Broker) error {
	registrations := []struct {
		method  string
		handler control.Handler
	}{
		{MethodRepositoryClone, func(ctx context.Context, payload json.RawMessage) (any, error) {
			var req RepositoryCloneRequest
			if json.Unmarshal(payload, &req) != nil {
				return nil, control.ErrInvalidArgument
			}
			result, err := repositories.Clone(ctx, req.ID, req.Remote, req.Branch)
			return result, translateError(err)
		}},
		{MethodWorkspaceCopy, func(ctx context.Context, payload json.RawMessage) (any, error) {
			var req WorkspaceCopyRequest
			if json.Unmarshal(payload, &req) != nil {
				return nil, control.ErrInvalidArgument
			}
			if len(req.Repositories) != 0 {
				if req.Repository != "" {
					return nil, control.ErrInvalidArgument
				}
				result, err := repositories.CopyWorkspaceSet(ctx, req.ID, req.Repositories)
				return result, translateError(err)
			}
			result, err := repositories.CopyWorkspace(ctx, req.ID, req.Repository)
			return result, translateError(err)
		}},
		{MethodGitConnect, func(ctx context.Context, payload json.RawMessage) (any, error) {
			req, err := decodeEnvironmentName(payload)
			if err != nil {
				return nil, err
			}
			return nil, translateError(broker.Connect(ctx, req.Environment))
		}},
		{MethodGitPending, func(context.Context, json.RawMessage) (any, error) { return broker.Pending(), nil }},
		{MethodGitDecide, func(ctx context.Context, payload json.RawMessage) (any, error) {
			var req GitDecisionRequest
			if json.Unmarshal(payload, &req) != nil {
				return nil, control.ErrInvalidArgument
			}
			return nil, translateError(broker.Decide(req.ID, req.Approved))
		}},
	}
	for _, registration := range registrations {
		if err := server.Register(registration.method, registration.handler); err != nil {
			return err
		}
	}
	return nil
}
func (c *Client) CloneRepository(ctx context.Context, request RepositoryCloneRequest) (gitrepo.Object, error) {
	var response gitrepo.Object
	err := c.wire.Call(ctx, MethodRepositoryClone, request, &response)
	return response, err
}
func (c *Client) CopyWorkspace(ctx context.Context, request WorkspaceCopyRequest) (gitrepo.Object, error) {
	var response gitrepo.Object
	err := c.wire.Call(ctx, MethodWorkspaceCopy, request, &response)
	return response, err
}
func (c *Client) ConnectGit(ctx context.Context, environment string) error {
	return c.wire.Call(ctx, MethodGitConnect, EnvironmentNameRequest{Environment: environment}, nil)
}
func (c *Client) PendingGit(ctx context.Context) ([]gitrepo.Proposal, error) {
	var response []gitrepo.Proposal
	err := c.wire.Call(ctx, MethodGitPending, nil, &response)
	return response, err
}
func (c *Client) DecideGit(ctx context.Context, id string, approved bool) error {
	return c.wire.Call(ctx, MethodGitDecide, GitDecisionRequest{ID: id, Approved: approved}, nil)
}
