package control

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

type Dialer func(context.Context) (net.Conn, error)

type Client struct {
	dial Dialer
}

func NewClient(dial Dialer) (*Client, error) {
	if dial == nil {
		return nil, ErrInvalidArgument
	}
	return &Client{dial: dial}, nil
}

func (c *Client) Call(ctx context.Context, method string, request, response any) error {
	if c == nil || c.dial == nil || strings.TrimSpace(method) == "" {
		return ErrInvalidArgument
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	conn = bindContext(ctx, conn)
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	payload, err := marshalPayload(request)
	if err != nil {
		return err
	}
	if err := writeJSONLine(conn, requestEnvelope{Version: ProtocolVersion, Method: method, Payload: payload}); err != nil {
		return fmt.Errorf("write control request: %w", err)
	}
	wire, _, err := readResponse(conn)
	if err != nil {
		return err
	}
	if err := validateResponse(wire); err != nil {
		return err
	}
	if response == nil || len(wire.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(wire.Payload, response); err != nil {
		return fmt.Errorf("decode control response: %w", err)
	}
	return nil
}

// OpenStream opens the legacy raw byte-stream mode. It deliberately retains
// EOF-only completion semantics for compatibility with older peers.
func (c *Client) OpenStream(ctx context.Context, method string, request any) (net.Conn, error) {
	conn, _, err := c.openStream(ctx, method, request, false)
	return conn, err
}

// OpenSession opens a raw byte stream while negotiating an independent
// completion/control identity. If the peer is older and does not return a
// session id, it safely falls back to the legacy EOF-only stream behavior.
func (c *Client) OpenSession(ctx context.Context, method string, request any) (net.Conn, error) {
	conn, response, err := c.openStream(ctx, method, request, true)
	if err != nil {
		return nil, err
	}
	if response.SessionID == "" {
		return conn, nil
	}
	return &sessionConn{
		Conn:   conn,
		client: c,
		id:     response.SessionID,
		ctx:    ctx,
	}, nil
}

func (c *Client) openStream(ctx context.Context, method string, request any, managedSession bool) (net.Conn, responseEnvelope, error) {
	if c == nil || c.dial == nil || strings.TrimSpace(method) == "" {
		return nil, responseEnvelope{}, ErrInvalidArgument
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, responseEnvelope{}, err
	}
	conn = bindContext(ctx, conn)
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			conn.Close()
			return nil, responseEnvelope{}, err
		}
	}
	payload, err := marshalPayload(request)
	if err != nil {
		conn.Close()
		return nil, responseEnvelope{}, err
	}
	if err := writeJSONLine(conn, requestEnvelope{
		Version: ProtocolVersion,
		Method:  method,
		Stream:  true,
		Session: managedSession,
		Payload: payload,
	}); err != nil {
		conn.Close()
		return nil, responseEnvelope{}, fmt.Errorf("write control stream request: %w", err)
	}
	wire, reader, err := readResponse(conn)
	if err != nil {
		conn.Close()
		return nil, responseEnvelope{}, err
	}
	if err := validateResponse(wire); err != nil {
		conn.Close()
		return nil, responseEnvelope{}, err
	}
	if _, ok := ctx.Deadline(); !ok {
		_ = conn.SetDeadline(noDeadline)
	}
	return &bufferedConn{Conn: conn, reader: reader}, wire, nil
}

type contextConn struct {
	net.Conn
	stop func() bool
}

func bindContext(ctx context.Context, conn net.Conn) net.Conn {
	if ctx == nil || ctx.Done() == nil {
		return conn
	}
	bound := &contextConn{Conn: conn}
	bound.stop = context.AfterFunc(ctx, func() { _ = conn.Close() })
	return bound
}

func (c *contextConn) Close() error {
	if c.stop != nil {
		c.stop()
	}
	return c.Conn.Close()
}

func marshalPayload(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode control payload: %w", err)
	}
	return payload, nil
}

func readResponse(conn net.Conn) (responseEnvelope, *bufio.Reader, error) {
	reader := bufio.NewReader(conn)
	line, err := readEnvelopeLine(reader)
	if err != nil {
		return responseEnvelope{}, nil, fmt.Errorf("read control response: %w", err)
	}
	var response responseEnvelope
	if err := json.Unmarshal(line, &response); err != nil {
		return responseEnvelope{}, nil, fmt.Errorf("decode control response: %w", ErrProtocol)
	}
	return response, reader, nil
}
