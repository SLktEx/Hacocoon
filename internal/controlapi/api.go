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
	MethodPing              = "system.ping"
	MethodEnvironmentCreate = "environment.create"
	MethodEnvironmentList   = "environment.list"
	MethodEnvironmentStatus = "environment.status"
	MethodEnvironmentExec   = "environment.exec"
	MethodEnvironmentShell  = "environment.shell"
	MethodEnvironmentDelete = "environment.delete"
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
	if err := server.Register(MethodEnvironmentExec, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request EnvironmentExecRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, control.NewStatusError("invalid_argument", "invalid environment exec request")
		}
		if strings.TrimSpace(request.Environment) == "" || len(request.Argv) == 0 {
			return nil, control.NewStatusError("invalid_argument", "environment and argv are required")
		}
		result, err := environments.Exec(ctx, request.Environment, core.ExecutionRequest{Argv: request.Argv})
		if err != nil {
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
