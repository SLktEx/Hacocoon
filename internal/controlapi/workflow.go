package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/workflow"
)

const MethodWorkflow = "workspace.workflow"

type WorkflowRequest struct {
	Operation string                `json:"operation"`
	Prepare   *workflow.PrepareSpec `json:"prepare,omitempty"`
	Open      *workflow.OpenSpec    `json:"open,omitempty"`
	Reference *workflow.Reference   `json:"reference,omitempty"`
	Target    string                `json:"target,omitempty"`
}
type WorkflowResponse struct {
	Reference *workflow.Reference  `json:"reference,omitempty"`
	Open      *workflow.OpenResult `json:"open,omitempty"`
	Fork      *workflow.ForkResult `json:"fork,omitempty"`
	Error     *responseStatus      `json:"error,omitempty"`
}
type workflowService interface {
	Reference(context.Context, string) (workflow.Reference, error)
	Prepare(context.Context, workflow.PrepareSpec) (workflow.Reference, error)
	Open(context.Context, workflow.OpenSpec) (workflow.OpenResult, error)
	Fork(context.Context, workflow.Reference, string) (workflow.ForkResult, error)
}

func RegisterWorkflow(server *control.Server, service workflowService) error {
	return server.Register(MethodWorkflow, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req WorkflowRequest
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF {
			return nil, translateError(core.ErrInvalidArgument)
		}
		valid := false
		switch req.Operation {
		case "prepare":
			valid = req.Prepare != nil && req.Open == nil && req.Reference == nil && req.Target == ""
		case "open":
			valid = req.Open != nil && req.Prepare == nil && req.Reference == nil && req.Target == ""
		case "reference":
			valid = req.Reference != nil && req.Reference.Workspace == "" && req.Prepare == nil && req.Open == nil && req.Target == ""
		case "fork":
			valid = req.Reference != nil && req.Reference.Workspace != "" && req.Prepare == nil && req.Open == nil && req.Target != ""
		}
		if !valid {
			return nil, translateError(core.ErrInvalidArgument)
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		var response WorkflowResponse
		var err error
		switch req.Operation {
		case "prepare":
			ref, e := service.Prepare(ctx, *req.Prepare)
			response.Reference, err = &ref, e
		case "reference":
			ref, e := service.Reference(ctx, req.Reference.Name)
			response.Reference, err = &ref, e
		case "open":
			result, e := service.Open(ctx, *req.Open)
			response.Open, err = &result, e
		case "fork":
			result, e := service.Fork(ctx, *req.Reference, req.Target)
			response.Fork, err = &result, e
		}
		response.Error = statusFromError(err)
		return response, nil
	})
}
func (c *Client) WorkspaceWorkflow(ctx context.Context, req WorkflowRequest) (WorkflowResponse, error) {
	var response WorkflowResponse
	if err := c.wire.Call(ctx, MethodWorkflow, req, &response); err != nil {
		return response, err
	}
	return response, responseError(response.Error)
}
