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
	MethodPing             = "system.ping"
	MethodEnvironmentExec  = "environment.exec"
	MethodEnvironmentShell = "environment.shell"
)

type EnvironmentExecRequest struct {
	Environment string   `json:"environment"`
	Argv        []string `json:"argv"`
}

type EnvironmentShellRequest struct {
	Environment string `json:"environment"`
}

type environmentService interface {
	Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	ShellStream(context.Context, string, io.Reader, io.Writer, io.Writer) error
}

func Register(server *control.Server, environments environmentService) error {
	if server == nil || environments == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodPing, func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"protocol_version": control.ProtocolVersion}, nil
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
	if err := server.RegisterStream(MethodEnvironmentShell, func(ctx context.Context, payload json.RawMessage, conn net.Conn) error {
		var request EnvironmentShellRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Environment) == "" {
			return control.NewStatusError("invalid_argument", "environment is required")
		}
		return environments.ShellStream(ctx, request.Environment, conn, conn, conn)
	}); err != nil {
		return err
	}
	return nil
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
