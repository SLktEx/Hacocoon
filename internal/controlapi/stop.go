package controlapi

import (
	"context"
	"encoding/json"

	"github.com/SLktEx/Hacocoon/internal/control"
)

const MethodEnvironmentStop = "environment.stop"

func RegisterStop(server *control.Server, environments interface {
	Stop(context.Context, string) error
}) error {
	return server.Register(MethodEnvironmentStop, func(ctx context.Context, payload json.RawMessage) (any, error) {
		request, err := decodeEnvironmentName(payload)
		if err != nil {
			return nil, err
		}
		return nil, translateError(environments.Stop(ctx, request.Environment))
	})
}

func (c *Client) StopEnvironment(ctx context.Context, environment string) error {
	return c.wire.Call(ctx, MethodEnvironmentStop, EnvironmentNameRequest{Environment: environment}, nil)
}
