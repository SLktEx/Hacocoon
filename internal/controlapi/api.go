package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	MethodPing                   = "system.ping"
	MethodEnvironmentCreate      = "environment.create"
	MethodEnvironmentList        = "environment.list"
	MethodEnvironmentStatus      = "environment.status"
	MethodEnvironmentExec        = "environment.exec"
	MethodEnvironmentShell       = "environment.shell"
	MethodEnvironmentDelete      = "environment.delete"
	MethodEnvironmentConnections = "environment.connections"
	MethodEnvironmentForward     = "environment.forward"
	MethodEnvironmentUnforward   = "environment.unforward"
	MethodEnvironmentSSH         = "environment.ssh"
	MethodHostShell              = "host.shell"
)

type EnvironmentCreateRequest struct {
	Name          string                   `json:"name"`
	WorkspacePath string                   `json:"workspace_path"`
	AccessMode    core.WorkspaceAccessMode `json:"access_mode,omitempty"`
	Base          core.BaseName            `json:"base,omitempty"`
	Resources     core.ResourceBudget      `json:"resources,omitempty"`
}

type EnvironmentNameRequest struct {
	Environment string `json:"environment"`
}

type EnvironmentExecRequest struct {
	Environment string   `json:"environment"`
	Argv        []string `json:"argv"`
}

type EnvironmentForwardRequest struct {
	Environment string `json:"environment"`
	Protocol    string `json:"protocol,omitempty"`
	HostPort    int    `json:"host_port"`
	TargetPort  int    `json:"target_port"`
}

type EnvironmentConnectionRequest struct {
	Environment  string `json:"environment"`
	ConnectionID string `json:"connection_id"`
}

type EnvironmentSSHRequest struct {
	Environment string `json:"environment"`
	PublicKey   string `json:"public_key"`
	HostPort    int    `json:"host_port"`
}

type EnvironmentShellRequest = EnvironmentNameRequest

type environmentService interface {
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	List(context.Context) ([]core.Environment, error)
	Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	PrepareShellStream(context.Context, string) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error)
	Delete(context.Context, string) error
}

type clientService interface {
	Status(context.Context, string) (core.EnvironmentStatus, error)
	Connections(context.Context, string) ([]core.ClientConnection, error)
	Forward(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error)
	Unforward(context.Context, string, string) error
	SSH(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
}

type hostService interface {
	PrepareTrustedHostShellStream(context.Context) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error)
}

func Register(server *control.Server, environments environmentService, clients clientService) error {
	if server == nil || environments == nil || clients == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodPing, func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"protocol_version": control.ProtocolVersion}, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentCreate, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request EnvironmentCreateRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, control.NewStatusError("invalid_argument", "invalid environment create request")
		}
		if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.WorkspacePath) == "" {
			return nil, control.NewStatusError("invalid_argument", "name and workspace_path are required")
		}
		environment, err := environments.Create(ctx, core.EnvironmentSpec{
			Name:          request.Name,
			WorkspacePath: request.WorkspacePath,
			AccessMode:    request.AccessMode,
			Base:          request.Base,
			Resources:     request.Resources,
		})
		if err != nil {
			return nil, translateError(err)
		}
		return environment, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentList, func(ctx context.Context, _ json.RawMessage) (any, error) {
		environments, err := environments.List(ctx)
		if err != nil {
			return nil, translateError(err)
		}
		return environments, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentStatus, func(ctx context.Context, payload json.RawMessage) (any, error) {
		request, err := decodeEnvironmentName(payload)
		if err != nil {
			return nil, err
		}
		status, err := clients.Status(ctx, request.Environment)
		if err != nil {
			return nil, translateError(err)
		}
		return status, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentConnections, func(ctx context.Context, payload json.RawMessage) (any, error) {
		request, err := decodeEnvironmentName(payload)
		if err != nil {
			return nil, err
		}
		connections, err := clients.Connections(ctx, request.Environment)
		if err != nil {
			return nil, translateError(err)
		}
		if connections == nil {
			connections = []core.ClientConnection{}
		}
		return connections, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentForward, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request EnvironmentForwardRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" {
			return nil, control.NewStatusError("invalid_argument", "invalid environment forward request")
		}
		connection, err := clients.Forward(ctx, request.Environment, core.LocalPortRequest{
			Protocol:   request.Protocol,
			HostPort:   request.HostPort,
			TargetPort: request.TargetPort,
		})
		if err != nil {
			return nil, translateError(err)
		}
		return connection, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentUnforward, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request EnvironmentConnectionRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" || strings.TrimSpace(request.ConnectionID) == "" {
			return nil, control.NewStatusError("invalid_argument", "environment and connection_id are required")
		}
		if err := clients.Unforward(ctx, request.Environment, request.ConnectionID); err != nil {
			return nil, translateError(err)
		}
		return nil, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentSSH, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request EnvironmentSSHRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" || strings.TrimSpace(request.PublicKey) == "" {
			return nil, control.NewStatusError("invalid_argument", "environment and public_key are required")
		}
		connection, err := clients.SSH(ctx, request.Environment, core.SSHAccessRequest{
			PublicKey: request.PublicKey,
			HostPort:  request.HostPort,
		})
		if err != nil {
			return nil, translateError(err)
		}
		return connection, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentExec, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request EnvironmentExecRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, control.NewStatusError("invalid_argument", "invalid environment exec request")
		}
		if strings.TrimSpace(request.Environment) == "" || len(request.Argv) == 0 {
			return nil, control.NewStatusError("invalid_argument", "environment and argv are required")
		}
		result, err := environments.Exec(ctx, request.Environment, core.ExecutionRequest{Argv: request.Argv})
		if err != nil && result.ExitCode <= 0 {
			return nil, translateError(err)
		}
		return result, nil
	}); err != nil {
		return err
	}
	if err := server.RegisterStream(MethodEnvironmentShell, func(ctx context.Context, payload json.RawMessage) (control.Stream, error) {
		request, err := decodeEnvironmentName(payload)
		if err != nil {
			return nil, err
		}
		prepared, err := environments.PrepareShellStream(ctx, request.Environment)
		if err != nil {
			return nil, translateError(err)
		}
		return func(runCtx context.Context, conn net.Conn) error {
			return prepared(runCtx, conn, conn, conn)
		}, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodEnvironmentDelete, func(ctx context.Context, payload json.RawMessage) (any, error) {
		request, err := decodeEnvironmentName(payload)
		if err != nil {
			return nil, err
		}
		if err := environments.Delete(ctx, request.Environment); err != nil {
			return nil, translateError(err)
		}
		return nil, nil
	}); err != nil {
		return err
	}
	return nil
}

// RegisterHost adds controller-owned trusted Host operations without widening
// the Environment API registration surface. Bootstrap-only Host operations stay
// local to the Physical Host CLI; only the interactive shell is a client API.
func RegisterHost(server *control.Server, hosts hostService) error {
	if server == nil || hosts == nil {
		return control.ErrInvalidArgument
	}
	return server.RegisterStream(MethodHostShell, func(ctx context.Context, _ json.RawMessage) (control.Stream, error) {
		prepared, err := hosts.PrepareTrustedHostShellStream(ctx)
		if err != nil {
			return nil, translateError(err)
		}
		return func(runCtx context.Context, conn net.Conn) error {
			return prepared(runCtx, conn, conn, conn)
		}, nil
	})
}

func decodeEnvironmentName(payload json.RawMessage) (EnvironmentNameRequest, error) {
	var request EnvironmentNameRequest
	if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" {
		return EnvironmentNameRequest{}, control.NewStatusError("invalid_argument", "environment is required")
	}
	return request, nil
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	code := "internal"
	switch {
	case errors.Is(err, core.ErrInvalidArgument):
		code = "invalid_argument"
	case errors.Is(err, core.ErrNotFound):
		code = "not_found"
	case errors.Is(err, core.ErrAlreadyExists):
		code = "already_exists"
	case errors.Is(err, core.ErrUnsupported):
		code = "unsupported"
	case errors.Is(err, core.ErrRuntimeUnavailable), errors.Is(err, core.ErrStorageUnavailable):
		code = "unavailable"
	case errors.Is(err, core.ErrPolicyDenied), errors.Is(err, core.ErrApprovalDenied):
		code = "denied"
	case errors.Is(err, core.ErrWorkspaceBusy), errors.Is(err, core.ErrStorageBusy):
		code = "busy"
	case errors.Is(err, core.ErrIncompatibleState):
		code = "incompatible_state"
	case errors.Is(err, core.ErrRecoveryRequired):
		code = "recovery_required"
	}
	return control.NewStatusError(code, fmt.Sprint(err))
}
