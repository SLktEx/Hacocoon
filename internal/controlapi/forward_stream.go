package controlapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/streamio"
)

const MethodForwardPrepare = "environment.forward.prepare"
const MethodForwardStream = "environment.forward.stream"

type forwardService interface {
	PrepareTCPForward(context.Context, string, string, int) (core.EnvironmentTCPForward, error)
	DialTCPForward(context.Context, core.EnvironmentTCPForward) (net.Conn, error)
}

type forwardReady struct {
	Ready bool            `json:"ready"`
	Error *responseStatus `json:"error,omitempty"`
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
			stop := context.AfterFunc(ctx, func() { _ = client.Close() })
			defer stop()
			upstream, err := service.DialTCPForward(ctx, target)
			if err != nil {
				_ = client.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_ = json.NewEncoder(client).Encode(forwardReady{Error: statusFromError(err)})
				return translateError(err)
			}
			defer upstream.Close()
			if err = client.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return err
			}
			if err = json.NewEncoder(client).Encode(forwardReady{Ready: true}); err != nil {
				return err
			}
			if err = client.SetWriteDeadline(time.Time{}); err != nil {
				return err
			}
			return streamio.Relay(ctx, client, upstream)
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
	conn, err := c.wire.OpenByteSession(ctx, MethodForwardStream, target)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReaderSize(conn, 4096)
	line, err := reader.ReadSlice('\n')
	var ready forwardReady
	if err == nil {
		err = json.Unmarshal(line, &ready)
	}
	if err == nil && ready.Error != nil {
		err = control.NewStatusError(ready.Error.Code, ready.Error.Message)
	}
	if err == nil && !ready.Ready {
		err = control.ErrProtocol
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &forwardStream{Conn: conn, reader: reader}, nil
}

type forwardStream struct {
	net.Conn
	reader *bufio.Reader
}

func (c *forwardStream) Read(b []byte) (int, error) { return c.reader.Read(b) }
func (c *forwardStream) CloseWrite() error {
	if half, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return half.CloseWrite()
	}
	return control.ErrProtocol
}

// Wait retains target failure separately from application EOF.
func (c *forwardStream) Wait(ctx context.Context) error {
	if session, ok := c.Conn.(interface{ Wait(context.Context) error }); ok {
		return session.Wait(ctx)
	}
	return io.ErrUnexpectedEOF
}
