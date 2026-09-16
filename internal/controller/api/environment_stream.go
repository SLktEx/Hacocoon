package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const MethodEnvironmentStream = "environment.stream"

type streamService interface {
	DialStream(context.Context, core.StreamTarget) (net.Conn, error)
}

// RegisterEnvironmentStreams belongs only on the private management endpoint.
// Preparation allocates nothing. Upstream creation begins only inside the owned
// stream callback, so failed transport acknowledgement cannot leak a socket.
func RegisterEnvironmentStreams(server *control.Server, service streamService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	return server.RegisterStream(MethodEnvironmentStream, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
		var target core.StreamTarget
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&target) != nil || !target.Valid() {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, client net.Conn) error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			return relayByteStream(ctx, client, func(ctx context.Context) (net.Conn, error) {
				return service.DialStream(ctx, target)
			})
		}, nil
	})
}

func (c *Client) OpenEnvironmentStream(ctx context.Context, target core.StreamTarget) (net.Conn, error) {
	return c.openByteStream(ctx, MethodEnvironmentStream, target, 100*time.Second)
}
