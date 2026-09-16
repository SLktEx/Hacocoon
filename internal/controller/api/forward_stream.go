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

const MethodForwardPrepare = "environment.forward.prepare"
const MethodForwardStream = "environment.forward.stream"

type forwardService interface {
	PrepareTCPForward(context.Context, string, string, int) (core.EnvironmentTCPForward, error)
	DialTCPForward(context.Context, core.EnvironmentTCPForward) (net.Conn, error)
}

// RegisterForwardStreams belongs only on the private management endpoint.
// Preparation allocates nothing. Upstream creation begins only inside the owned
// stream callback, so failed transport acknowledgement cannot leak a socket.
func RegisterForwardStreams(server *control.Server, service forwardService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodForwardPrepare, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request core.EnvironmentTCPForward
		if json.Unmarshal(payload, &request) != nil || request.Instance != "" {
			return nil, control.ErrInvalidArgument
		}
		target, err := service.PrepareTCPForward(ctx, request.Environment, request.Address, request.Port)
		return target, translateError(err)
	}); err != nil {
		return err
	}
	return server.RegisterStream(MethodForwardStream, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
		var target core.EnvironmentTCPForward
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&target) != nil || !core.ValidEnvironmentInstanceID(target.Instance) || !core.ValidForwardAddress(target.Address, target.Port) {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, client net.Conn) error {
			ctx, cancel := context.WithTimeout(ctx, time.Hour)
			defer cancel()
			return relayByteStream(ctx, client, func(ctx context.Context) (net.Conn, error) {
				return service.DialTCPForward(ctx, target)
			})
		}, nil
	})
}

func (c *Client) PrepareEnvironmentForward(ctx context.Context, name, address string, port int) (core.EnvironmentTCPForward, error) {
	var target core.EnvironmentTCPForward
	err := c.wire.Call(ctx, MethodForwardPrepare, core.EnvironmentTCPForward{Environment: name, Address: address, Port: port}, &target)
	if err == nil && (target.Environment != name || target.Address != address || target.Port != port || !core.ValidEnvironmentInstanceID(target.Instance)) {
		err = control.ErrProtocol
	}
	return target, err
}

func (c *Client) OpenEnvironmentForward(ctx context.Context, target core.EnvironmentTCPForward) (net.Conn, error) {
	return c.openByteStream(ctx, MethodForwardStream, target, 10*time.Second)
}
