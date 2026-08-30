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

func (c *Client) OpenStream(ctx context.Context, method string, request any) (net.Conn, error) {
	if c == nil || c.dial == nil || strings.TrimSpace(method) == "" {
		return nil, ErrInvalidArgument
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			conn.Close()
			return nil, err
		}
	}
	payload, err := marshalPayload(request)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := writeJSONLine(conn, requestEnvelope{Version: ProtocolVersion, Method: method, Stream: true, Payload: payload}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write control stream request: %w", err)
	}
	wire, reader, err := readResponse(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := validateResponse(wire); err != nil {
		conn.Close()
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		_ = conn.SetDeadline(noDeadline)
	}
	return &bufferedConn{Conn: conn, reader: reader}, nil
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
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return responseEnvelope{}, nil, fmt.Errorf("read control response: %w", err)
	}
	var response responseEnvelope
	if err := json.Unmarshal(line, &response); err != nil {
		return responseEnvelope{}, nil, fmt.Errorf("decode control response: %w", ErrProtocol)
	}
	return response, reader, nil
}
