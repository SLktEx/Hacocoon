package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const MethodSetup = "system.setup"
const setupTimeout = 15 * time.Minute

type setupService interface {
	SetupHost(context.Context, hostsetup.Update) error
}

// RegisterSetup keeps bootstrap under the same controller authority as normal
// operations. There is no second local composition path in the product client.
func RegisterSetup(server *control.Server, service setupService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	active := make(chan struct{}, 1)
	return server.Register(MethodSetup, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var update hostsetup.Update
		if len(bytes.TrimSpace(payload)) != 0 {
			decoder := json.NewDecoder(bytes.NewReader(payload))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&update); err != nil {
				return nil, control.NewStatusError("invalid_argument", "invalid Host setup options")
			}
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				return nil, control.NewStatusError("invalid_argument", "invalid Host setup options")
			}
		}
		if err := update.Validate(); err != nil {
			return nil, control.NewStatusError("invalid_argument", err.Error())
		}

		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			return nil, control.NewStatusError("busy", "Host setup is already running")
		}
		ctx, cancel := context.WithTimeout(ctx, setupTimeout)
		defer cancel()
		if err := service.SetupHost(ctx, update); err != nil || ctx.Err() != nil {
			// The provider error may contain arbitrary guest/backend output. Record the
			// owning failure boundary without forwarding that output to logs or clients.
			if errors.Is(err, hostsetup.ErrCustomizationFailed) {
				logging.Root().ErrorContext(ctx, "Trusted Host customization failed", "component", "bootstrap", "operation", "setup")
				return nil, control.NewStatusError("customization_failed", "Host prepared, but customization failed; update the script and rerun haco setup")
			}
			logging.Root().ErrorContext(ctx, "Trusted Host setup failed", "component", "bootstrap", "operation", "setup")
			return nil, control.NewStatusError("setup_failed", "Host setup failed; run haco doctor, then rerun the installer")
		}
		return PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
	})
}

func (c *Client) SetupHost(ctx context.Context, update hostsetup.Update) error {
	var response PingResponse
	if err := c.wire.Call(ctx, MethodSetup, update, &response); err != nil {
		return err
	}
	if response.ProtocolVersion != control.ProtocolVersion {
		return control.ErrProtocol
	}
	return nil
}
