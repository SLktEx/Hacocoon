package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const MethodConfigurationRead = "configuration.read"
const MethodConfigurationReplace = "configuration.replace"

type configurationService interface {
	Snapshot(context.Context) (capability.PolicySnapshot, error)
	Replace(context.Context, capability.PolicySnapshot) (capability.PolicySnapshot, error)
}

// Configuration is exposed only on the existing trusted management transport.
// It is never registered on the guest Git socket or notification HTTP bridge.
func RegisterConfiguration(server *control.Server, service configurationService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodConfigurationRead, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if len(payload) != 0 && string(payload) != "null" && string(payload) != "{}" {
			return nil, control.ErrInvalidArgument
		}
		result, err := service.Snapshot(ctx)
		return result, configurationError(err)
	}); err != nil {
		return err
	}
	return server.Register(MethodConfigurationReplace, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var edit capability.PolicySnapshot
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(&edit) != nil || d.Decode(new(any)) != io.EOF {
			return nil, control.ErrInvalidArgument
		}
		result, err := service.Replace(ctx, edit)
		return result, configurationError(err)
	})
}

func configurationError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, core.ErrIncompatibleState):
		return control.NewStatusError("incompatible_state", "configuration changed or storage is unsafe; read it again before retrying")
	case errors.Is(err, core.ErrInvalidArgument):
		return control.NewStatusError("invalid_argument", "invalid configuration document or revision")
	case errors.Is(err, core.ErrRecoveryRequired):
		return control.NewStatusError("recovery_required", "configuration may be saved but audit is incomplete; inspect configuration before retrying")
	case errors.Is(err, core.ErrUnsupported):
		return control.NewStatusError("unsupported", "configuration management is unavailable")
	default:
		return control.NewStatusError("internal", "configuration operation failed; inspect configuration before retrying")
	}
}

func (c *Client) ReadConfiguration(ctx context.Context) (capability.PolicySnapshot, error) {
	var result capability.PolicySnapshot
	err := c.wire.Call(ctx, MethodConfigurationRead, nil, &result)
	return result, err
}
func (c *Client) ReplaceConfiguration(ctx context.Context, edit capability.PolicySnapshot) (capability.PolicySnapshot, error) {
	var result capability.PolicySnapshot
	err := c.wire.Call(ctx, MethodConfigurationReplace, edit, &result)
	return result, err
}
