package controlapi

import (
	"context"
	"encoding/json"

	"github.com/SLktEx/Hacocoon/internal/control"
)

const MethodEnvironmentStart = "environment.start"

func RegisterStart(server *control.Server, environments interface {
	Start(context.Context, string) error
}) error {
	return server.Register(MethodEnvironmentStart, func(ctx context.Context, payload json.RawMessage) (any, error) {
		request, err := decodeEnvironmentName(payload)
		if err != nil {
			return nil, err
		}
		return nil, translateError(environments.Start(ctx, request.Environment))
	})
}

func (c *Client) StartEnvironment(ctx context.Context, environment string) error {
	return c.wire.Call(ctx, MethodEnvironmentStart, EnvironmentNameRequest{Environment: environment}, nil)
}
