package controlapi

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const MethodEnvironmentRemove = "environment.remove"

type RemoveRequest struct {
	Environment string `json:"environment"`
	Force       bool   `json:"force,omitempty"`
}

func RegisterRemove(server *control.Server, service interface {
	DeleteUser(context.Context, string, bool) error
}) error {
	return server.Register(MethodEnvironmentRemove, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var r RemoveRequest
		if strictDecode(payload, &r) != nil || core.ValidateEnvironmentName(r.Environment) != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		return nil, translateError(service.DeleteUser(ctx, r.Environment, r.Force))
	})
}
func (c *Client) RemoveEnvironment(ctx context.Context, name string, force bool) error {
	return c.wire.Call(ctx, MethodEnvironmentRemove, RemoveRequest{name, force}, nil)
}
