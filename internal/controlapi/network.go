package controlapi

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/modules/standard/networkrelay"
)

const MethodNetworkList = "network.list"
const MethodNetworkRevoke = "network.revoke"

func RegisterNetwork(server *control.Server, service *networkrelay.Service) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodNetworkList, func(context.Context, json.RawMessage) (any, error) { return service.List(), nil }); err != nil {
		return err
	}
	return server.Register(MethodNetworkRevoke, func(_ context.Context, payload json.RawMessage) (any, error) {
		var request struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(payload, &request) != nil || len(request.ID) != 32 {
			return nil, control.ErrInvalidArgument
		}
		return nil, translateError(service.Revoke(request.ID))
	})
}
func (c *Client) ListNetworkConnections(ctx context.Context) ([]networkrelay.Session, error) {
	var result []networkrelay.Session
	err := c.wire.Call(ctx, MethodNetworkList, nil, &result)
	return result, err
}
func (c *Client) RevokeNetworkConnection(ctx context.Context, id string) error {
	return c.wire.Call(ctx, MethodNetworkRevoke, map[string]string{"id": id}, nil)
}

const MethodNetworkRule = "network.rule"

func RegisterNetworkRules(server *control.Server, service *networkrelay.Service, configuration networkrelay.PolicyEditor) error {
	return server.Register(MethodNetworkRule, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var spec networkrelay.RuleSpec
		if json.Unmarshal(payload, &spec) != nil {
			return nil, control.ErrInvalidArgument
		}
		rule, err := service.AddRule(ctx, configuration, spec)
		return rule, translateError(err)
	})
}
func (c *Client) AddNetworkRule(ctx context.Context, spec networkrelay.RuleSpec) (capability.PolicyRule, error) {
	var result capability.PolicyRule
	err := c.wire.Call(ctx, MethodNetworkRule, spec, &result)
	return result, err
}
