package controlapi

import (
	"context"
	"encoding/json"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

const MethodGitStatus = "git.status"

type GitBrokerStatusResponse struct {
	Applicable bool `json:"applicable"`
	Connected  bool `json:"connected"`
}

func gitStatusHandler(broker *gitrepo.Broker) control.Handler {
	return func(ctx context.Context, payload json.RawMessage) (any, error) {
		req, err := decodeEnvironmentName(payload)
		if err != nil {
			return nil, err
		}
		applicable, connected, err := broker.Status(ctx, req.Environment)
		if err != nil {
			return nil, translateError(err)
		}
		return GitBrokerStatusResponse{Applicable: applicable, Connected: connected}, nil
	}
}

func (c *Client) GitBrokerStatus(ctx context.Context, environment string) (GitBrokerStatusResponse, error) {
	var response GitBrokerStatusResponse
	err := c.wire.Call(ctx, MethodGitStatus, EnvironmentNameRequest{Environment: environment}, &response)
	return response, err
}
