package controlapi

import (
	"context"
	"net"
	"os"
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

func (c *Client) EnvironmentConnections(ctx context.Context, environment string) ([]core.ClientConnection, error) {
	var response []core.ClientConnection
	err := c.wire.Call(ctx, MethodEnvironmentConnections, EnvironmentNameRequest{Environment: environment}, &response)
	if response == nil && err == nil {
		response = []core.ClientConnection{}
	}
	return response, err
}

func (c *Client) ForwardEnvironment(ctx context.Context, environment string, request core.LocalPortRequest) (core.ClientConnection, error) {
	var response core.ClientConnection
	err := c.wire.Call(ctx, MethodEnvironmentForward, EnvironmentForwardRequest{
		Environment: environment,
		Protocol:    request.Protocol,
		HostPort:    request.HostPort,
		TargetPort:  request.TargetPort,
	}, &response)
	return response, err
}

func (c *Client) UnforwardEnvironment(ctx context.Context, environment, connectionID string) error {
	return c.wire.Call(ctx, MethodEnvironmentUnforward, EnvironmentConnectionRequest{
		Environment:  environment,
		ConnectionID: connectionID,
	}, nil)
}

func (c *Client) PrepareEnvironmentSSH(ctx context.Context, environment string, request core.SSHAccessRequest) (core.ClientConnection, error) {
	var response core.ClientConnection
	err := c.wire.Call(ctx, MethodEnvironmentSSH, EnvironmentSSHRequest{
		Environment: environment,
		PublicKey:   request.PublicKey,
		HostPort:    request.HostPort,
	}, &response)
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
	return c.wire.OpenSession(ctx, MethodEnvironmentShell, EnvironmentShellRequest{
		Environment: environment,
		Terminal:    currentTerminalMetadata(),
	})
}

func (c *Client) DeleteEnvironment(ctx context.Context, environment string) error {
	return c.wire.Call(ctx, MethodEnvironmentDelete, EnvironmentNameRequest{Environment: environment}, nil)
}

func (c *Client) OpenTrustedHostShell(ctx context.Context) (net.Conn, error) {
	return c.wire.OpenSession(ctx, MethodHostShell, HostShellRequest{Terminal: currentTerminalMetadata()})
}

func currentTerminalMetadata() TerminalMetadata {
	return TerminalMetadata{
		Term:      os.Getenv("TERM"),
		ColorTerm: os.Getenv("COLORTERM"),
	}
}
