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

const MethodEnvironmentStream = "environment.stream"

type streamService interface {
	DialStream(context.Context, core.StreamTarget) (net.Conn, error)
}

type forwardReady struct {
	Ready bool            `json:"ready"`
	Error *responseStatus `json:"error,omitempty"`
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
			stop := context.AfterFunc(ctx, func() { _ = client.Close() })
			defer stop()
			upstream, err := service.DialStream(ctx, target)
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

func (c *Client) OpenEnvironmentStream(ctx context.Context, target core.StreamTarget) (net.Conn, error) {
	ctx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(100*time.Second, cancel)
	defer timer.Stop()
	conn, err := c.wire.OpenByteSession(ctx, MethodEnvironmentStream, target)
	if err != nil {
		cancel()
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(100 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
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
		cancel()
		return nil, err
	}
	return &forwardStream{Conn: conn, reader: reader, cancel: cancel}, nil
}

type forwardStream struct {
	cancel context.CancelFunc
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

func (c *forwardStream) Close() error { defer c.cancel(); return c.Conn.Close() }
