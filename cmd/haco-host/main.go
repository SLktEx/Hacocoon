package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/terminalbridge"
)

type controllerClient interface {
	Ping(context.Context) (controlapi.PingResponse, error)
	CreateEnvironment(context.Context, controlapi.EnvironmentCreateRequest) (core.Environment, error)
	ListEnvironments(context.Context) ([]core.Environment, error)
	EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error)
	ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error)
	OpenEnvironmentShell(context.Context, string) (net.Conn, error)
	DeleteEnvironment(context.Context, string) error
}

type commandExitError struct{ code int }

func (e commandExitError) Error() string { return fmt.Sprintf("command exited %d", e.code) }
func (e commandExitError) ExitCode() int { return e.code }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fail(err)
	}
	if err := dispatch(ctx, client, os.Args[1:]); err != nil {
		fail(err)
	}
}

func dispatch(ctx context.Context, client controllerClient, args []string) error {
	if client == nil || len(args) == 0 {
		usage()
		return core.ErrInvalidArgument
	}
	switch args[0] {
	case "env":
		return envCommand(ctx, client, args[1:])
	case "doctor":
		return doctorCommand(ctx, client, args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func envCommand(ctx context.Context, client controllerClient, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco-host env <list|create|status|exec|shell|delete> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "list":
		return envListCommand(ctx, client, args[1:])
	case "create":
		return envCreateCommand(ctx, client, args[1:])
	case "status":
		return envStatusCommand(ctx, client, args[1:])
	case "exec":
		return envExecCommand(ctx, client, args[1:])
	case "shell":
		return envShellCommand(ctx, client, args[1:], os.Stdin, os.Stdout)
	case "delete":
		return envDeleteCommand(ctx, client, args[1:])
	default:
		return fmt.Errorf("unknown env command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func envListCommand(ctx context.Context, client controllerClient, args []string) error {
	jsonOutput := false
	if len(args) == 1 && args[0] == "--json" {
		jsonOutput = true
	} else if len(args) != 0 {
		return fmt.Errorf("usage: haco-host env list [--json]: %w", core.ErrInvalidArgument)
	}
	environments, err := client.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(environments)
	}
	for _, environment := range environments {
		fmt.Printf("%s\t%s\t%s\n", environment.Name, environment.AccessMode, environment.Workspace.Path)
	}
	return nil
}

func envCreateCommand(ctx context.Context, client controllerClient, args []string) error {
	request, err := parseCreateRequest(args)
	if err != nil {
		return err
	}
	environment, err := client.CreateEnvironment(ctx, request)
	if err != nil {
		return err
	}
	fmt.Printf("%s\t%s\t%s\n", environment.Name, environment.Workspace.Path, environment.AccessMode)
	return nil
}

func parseCreateRequest(args []string) (controlapi.EnvironmentCreateRequest, error) {
	usageError := func() error {
		return fmt.Errorf("usage: haco-host env create [--read-only] [--base <base>] [--cpu <n|unlimited>] [--memory <size|unlimited>] [--pids <n|unlimited>] [--root-size <size|unlimited>] --workspace <path> <environment>: %w", core.ErrInvalidArgument)
	}
	if len(args) < 3 {
		return controlapi.EnvironmentCreateRequest{}, usageError()
	}
	request := controlapi.EnvironmentCreateRequest{AccessMode: core.WorkspaceReadWrite}
	readOnlySeen := false
	for len(args) > 1 {
		switch args[0] {
		case "--read-only":
			if readOnlySeen {
				return controlapi.EnvironmentCreateRequest{}, usageError()
			}
			readOnlySeen = true
			request.AccessMode = core.WorkspaceReadOnly
			args = args[1:]
		case "--base":
			if len(args) < 3 || request.Base != "" {
				return controlapi.EnvironmentCreateRequest{}, usageError()
			}
			request.Base = core.BaseName(args[1])
			args = args[2:]
		case "--cpu", "--memory", "--pids", "--root-size":
			if len(args) < 3 {
				return controlapi.EnvironmentCreateRequest{}, usageError()
			}
			if err := setResourceOption(&request.Resources, args[0], args[1]); err != nil {
				return controlapi.EnvironmentCreateRequest{}, err
			}
			args = args[2:]
		case "--workspace":
			if len(args) < 3 || request.WorkspacePath != "" {
				return controlapi.EnvironmentCreateRequest{}, usageError()
			}
			request.WorkspacePath = args[1]
			args = args[2:]
		default:
			if len(args) != 1 {
				return controlapi.EnvironmentCreateRequest{}, fmt.Errorf("unknown create option %q: %w", args[0], core.ErrInvalidArgument)
			}
		}
	}
	if len(args) != 1 || strings.TrimSpace(request.WorkspacePath) == "" {
		return controlapi.EnvironmentCreateRequest{}, usageError()
	}
	request.Name = args[0]
	return request, nil
}

func envStatusCommand(ctx context.Context, client controllerClient, args []string) error {
	if len(args) < 1 || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		return fmt.Errorf("usage: haco-host env status <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	status, err := client.EnvironmentStatus(ctx, args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		return json.NewEncoder(os.Stdout).Encode(status)
	}
	fmt.Printf("name: %s\nstate: %s\nruntime: %s\nworkspace: %s\naccess: %s\n",
		status.Environment.Name, status.State, status.Environment.RuntimeRef,
		status.Environment.Workspace.Path, status.Environment.AccessMode)
	if status.Environment.Base != nil {
		fmt.Printf("base: %s\nbase-revision: %s\n", status.Environment.Base.Name, status.Environment.Base.Revision)
	}
	fmt.Printf("cpu: %s\nmemory-bytes: %s\npids: %s\nroot-bytes: %s\n",
		resourceLimitText(status.Environment.Resources.CPU),
		resourceLimitText(status.Environment.Resources.MemoryBytes),
		resourceLimitText(status.Environment.Resources.PIDs),
		resourceLimitText(status.Environment.Resources.RootBytes))
	return nil
}

func envExecCommand(ctx context.Context, client controllerClient, args []string) error {
	if len(args) < 3 || args[1] != "--" {
		return fmt.Errorf("usage: haco-host env exec <environment> -- <command...>: %w", core.ErrInvalidArgument)
	}
	result, err := client.ExecEnvironment(ctx, args[0], args[2:])
	fmt.Print(result.Stdout)
	fmt.Fprint(os.Stderr, result.Stderr)
	if err != nil {
		return err
	}
	if result.ExitCode > 0 {
		return commandExitError{code: result.ExitCode}
	}
	return nil
}

func envShellCommand(ctx context.Context, client controllerClient, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco-host env shell <environment>: %w", core.ErrInvalidArgument)
	}
	stream, err := client.OpenEnvironmentShell(ctx, args[0])
	if err != nil {
		return err
	}
	return terminalbridge.Bridge(ctx, stream, stdin, stdout)
}

func envDeleteCommand(ctx context.Context, client controllerClient, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco-host env delete <environment>: %w", core.ErrInvalidArgument)
	}
	return client.DeleteEnvironment(ctx, args[0])
}

func doctorCommand(ctx context.Context, client controllerClient, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: haco-host doctor: %w", core.ErrInvalidArgument)
	}
	response, err := client.Ping(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Hacocoon logical Host client")
	fmt.Printf("controller: %s\nprotocol-version: %d\n", control.SocketPath(), response.ProtocolVersion)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: haco-host <env|doctor>")
}

func fail(err error) {
	code := 1
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) && exitCoder.ExitCode() > 0 {
		code = exitCoder.ExitCode()
	}
	message := strings.TrimSpace(err.Error())
	if message != "" {
		fmt.Fprintln(os.Stderr, "haco-host:", message)
	}
	os.Exit(code)
}
