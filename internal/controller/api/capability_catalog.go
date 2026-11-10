package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

const MethodConfigurationCapabilities = "configuration.capabilities"

type capabilityCatalogService interface {
	CapabilityCatalog(context.Context) (capability.CapabilityCatalog, error)
}

// RegisterCapabilityCatalog adds metadata discovery only to the trusted Host
// management endpoint. Guest/notification endpoints must not register it.
func RegisterCapabilityCatalog(server *control.Server, service capabilityCatalogService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	return server.Register(MethodConfigurationCapabilities, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if len(payload) != 0 {
			decoder := json.NewDecoder(bytes.NewReader(payload))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&struct{}{}) != nil || decoder.Decode(new(any)) != io.EOF {
				return nil, control.ErrInvalidArgument
			}
		}
		catalog, err := service.CapabilityCatalog(ctx)
		if err != nil || capability.ValidateCapabilityCatalog(catalog) != nil {
			return nil, control.NewStatusError("unavailable", "capability metadata is unavailable")
		}
		return catalog, nil
	})
}

func (c *Client) CapabilityCatalog(ctx context.Context) (capability.CapabilityCatalog, error) {
	var result capability.CapabilityCatalog
	if err := c.wire.Call(ctx, MethodConfigurationCapabilities, nil, &result); err != nil {
		return capability.CapabilityCatalog{}, err
	}
	if err := capability.ValidateCapabilityCatalog(result); err != nil {
		return capability.CapabilityCatalog{}, control.ErrProtocol
	}
	return result, nil
}
