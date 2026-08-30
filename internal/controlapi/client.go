package controlapi

import (
	"context"
	"net"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type PingResponse struct {
	ProtocolVersion int `json:"protocol_version"`
}

type Client struct {
	wire *control.Client
}

func NewClient(path string) (*Client, error) {
	if strings.TrimSpace(path) == "" {
		return nil, control.ErrInvalidArgument
	}
	wire, err := control.NewClient(control.UnixDialer(path))
	if err != nil {
		return nil, err
	}
	return &Client{wire: wire}, nil
}

func NewDefaultClient() (*Client, error) {
	return NewClient(control.SocketPath())
}

func (c *Client) Ping(ctx context.Context) (PingResponse, error) {
	var response PingResponse
	err := c.wire.Call(ctx, MethodPing, nil, &response)
	return response, err
}

func (c *Client) CreateEnvironment(ctx context.Context, request EnvironmentCreateRequest) (core.Environment, error) {
	var response core.Environment
	err := c.wire.Call(ctx, MethodEnvironmentCreate, request, &response)
	return response, err
}

func (c *Client) ListEnvironments(ctx context.Context) ([]core.Environment, error) {
	var response []core.Environment
	err := c.wire.Call(ctx, MethodEnvironmentList, nil, &response)
	return response, err
}

func (c *Client) EnvironmentStatus(ctx context.Context, environment string) (core.EnvironmentStatus, error) {
	var response core.EnvironmentStatus
	err := c.wire.Call(ctx, MethodEnvironmentStatus, EnvironmentNameRequest{Environment: environment}, &response)
	return response, err
}

func (c *Client) ExecEnvironment(ctx context.Context, environment string, argv []string) (core.ExecutionResult, error) {
	var response core.ExecutionResult
	err := c.wire.Call(ctx, MethodEnvironmentExec, EnvironmentExecRequest{
		Environment: environment,
		Argv:        append([]string(nil), argv...),
	}, &response)
	return response, err
}

func (c *Client) OpenEnvironmentShell(ctx context.Context, environment string) (net.Conn, error) {
	return c.wire.OpenStream(ctx, MethodEnvironmentShell, EnvironmentShellRequest{Environment: environment})
}

func (c *Client) DeleteEnvironment(ctx context.Context, environment string) error {
	return c.wire.Call(ctx, MethodEnvironmentDelete, EnvironmentNameRequest{Environment: environment}, nil)
}
