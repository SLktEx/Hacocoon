package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/recipes"
	"github.com/SLktEx/Hacocoon/modules/standard/projectsetup"
)

const MethodProjectSetup = "environment.setup"

type projectSetupService interface {
	Apply(context.Context, string, recipes.Update) (projectsetup.Result, error)
}
type ProjectSetupRequest struct {
	Environment string         `json:"environment"`
	Update      recipes.Update `json:"update"`
}
type ProjectSetupResponse struct {
	Result projectsetup.Result `json:"result"`
	Failed bool                `json:"failed"`
}

func RegisterProjectSetup(server *control.Server, service projectSetupService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	return server.Register(MethodProjectSetup, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request ProjectSetupRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return nil, control.ErrInvalidArgument
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF || strings.TrimSpace(request.Environment) == "" {
			return nil, control.ErrInvalidArgument
		}
		if err := request.Update.Validate(); err != nil {
			return nil, control.ErrInvalidArgument
		}
		ctx, cancel := context.WithTimeout(ctx, setupTimeout)
		defer cancel()
		result, err := service.Apply(ctx, request.Environment, request.Update)
		if err != nil {
			// Recipe and guest output belong only in explicit command results, never logs.
			logging.Root().ErrorContext(ctx, "Project setup failed", "component", "project_setup", "operation", "setup")
		}
		return ProjectSetupResponse{Result: result, Failed: err != nil}, nil
	})
}

func (c *Client) SetupProject(ctx context.Context, environment string, update recipes.Update) (ProjectSetupResponse, error) {
	var response ProjectSetupResponse
	err := c.wire.Call(ctx, MethodProjectSetup, ProjectSetupRequest{Environment: environment, Update: update}, &response)
	return response, err
}
