package controlapi

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const MethodSetup = "system.setup"
const setupTimeout = 15 * time.Minute

type setupService interface{ SetupHost(context.Context) error }

// RegisterSetup keeps bootstrap under the same controller authority as normal
// operations. There is no second local composition path in the product client.
func RegisterSetup(server *control.Server, service setupService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	active := make(chan struct{}, 1)
	return server.Register(MethodSetup, func(ctx context.Context, payload json.RawMessage) (any, error) {
		switch strings.TrimSpace(string(payload)) {
		case "", "null", "{}":
		default:
			return nil, control.NewStatusError("invalid_argument", "setup accepts no parameters")
		}
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			return nil, control.NewStatusError("busy", "Host setup is already running")
		}
		ctx, cancel := context.WithTimeout(ctx, setupTimeout)
		defer cancel()
		if err := service.SetupHost(ctx); err != nil || ctx.Err() != nil {
			// The provider error may contain arbitrary guest/backend output. Record the
			// owning failure boundary without forwarding that output to logs or clients.
			logging.Root().ErrorContext(ctx, "Trusted Host setup failed", "component", "bootstrap", "operation", "setup")
			return nil, control.NewStatusError("setup_failed", "Host setup failed; run haco doctor, then rerun the installer")
		}
		return PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
	})
}

func (c *Client) SetupHost(ctx context.Context) error {
	var response PingResponse
	if err := c.wire.Call(ctx, MethodSetup, nil, &response); err != nil {
		return err
	}
	if response.ProtocolVersion != control.ProtocolVersion {
		return control.ErrProtocol
	}
	return nil
}
