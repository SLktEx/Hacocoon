package controlapi

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/streamio"
)

type forwardReady struct {
	Ready bool            `json:"ready"`
	Error *responseStatus `json:"error,omitempty"`
}

// relayByteStream owns readiness and socket cleanup after the method's target
// validation. Each service retains its own authorization and lifetime contract.
func relayByteStream(ctx context.Context, client net.Conn, dial func(context.Context) (net.Conn, error)) error {
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	upstream, err := dial(ctx)
	if err != nil {
		_ = client.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_ = json.NewEncoder(client).Encode(forwardReady{Error: statusFromError(err)})
		return translateError(err)
	}
	defer func() { _ = upstream.Close() }()
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
}

// openByteStream consumes the bounded readiness envelope before exposing any
// application bytes. The preparation deadline ends here; caller cancellation
// remains effective for the full session and its separate completion calls.
func (c *Client) openByteStream(ctx context.Context, method string, target any, preparation time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(preparation, cancel)
	defer timer.Stop()
	conn, err := c.wire.OpenByteSession(ctx, method, target)
	if err != nil {
		cancel()
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
	// A fired timer must never publish a usable connection, including when its
	// cancellation callback is waiting to run concurrently with readiness.
	if !timer.Stop() && err == nil {
		err = context.DeadlineExceeded
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		_ = conn.Close()
		cancel()
		return nil, err
	}
	return &forwardStream{Conn: conn, reader: reader, cancel: cancel}, nil
}

type forwardStream struct {
	net.Conn
	reader *bufio.Reader
	cancel context.CancelFunc
}

func (c *forwardStream) Read(b []byte) (int, error) { return c.reader.Read(b) }
func (c *forwardStream) CloseWrite() error {
	if half, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return half.CloseWrite()
	}
	return control.ErrProtocol
}
func (c *forwardStream) Wait(ctx context.Context) error {
	if session, ok := c.Conn.(interface{ Wait(context.Context) error }); ok {
		return session.Wait(ctx)
	}
	return io.ErrUnexpectedEOF
}
func (c *forwardStream) Close() error { defer c.cancel(); return c.Conn.Close() }
