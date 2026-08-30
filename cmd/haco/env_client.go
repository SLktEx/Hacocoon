package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type environmentControllerClient interface {
	CreateEnvironment(context.Context, controlapi.EnvironmentCreateRequest) (core.Environment, error)
	ListEnvironments(context.Context) ([]core.Environment, error)
	EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error)
	ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error)
	OpenEnvironmentShell(context.Context, string) (net.Conn, error)
	DeleteEnvironment(context.Context, string) error
}

type environmentControllerFactory func() (environmentControllerClient, error)

// Environment client commands are intentionally intercepted before main
// initializes composition.Local(). This is the first general-client slice of
// `haco`: the same binary can execute `haco env ...` from the Physical Host or
// from trusted haco-host without inheriting local Incus/filesystem authority.
func init() {
	handled, err := handleEnvironmentClientArgs(
		context.Background(),
		os.Args[1:],
		newDefaultEnvironmentController,
		os.Stdin,
		os.Stdout,
		os.Stderr,
	)
	if !handled {
		return
	}
	if err != nil {
		code := 1
		var exitCoder interface{ ExitCode() int }
		if errors.As(err, &exitCoder) && exitCoder.ExitCode() > 0 {
			code = exitCoder.ExitCode()
		}
		if message := strings.TrimSpace(err.Error()); message != "" {
			fmt.Fprintln(os.Stderr, "haco:", message)
		}
		os.Exit(code)
	}
	os.Exit(0)
}

func newDefaultEnvironmentController() (environmentControllerClient, error) {
	return controlapi.NewDefaultClient()
}

func handleEnvironmentClientArgs(
	ctx context.Context,
	args []string,
	factory environmentControllerFactory,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) (bool, error) {
	if len(args) == 0 || args[0] != "env" {
		return false, nil
	}
	if factory == nil || stdin == nil || stdout == nil || stderr == nil {
		return true, core.ErrInvalidArgument
	}
	if len(args) < 2 {
		return true, fmt.Errorf("usage: haco env <list|create|status|exec|shell|delete> ...: %w", core.ErrInvalidArgument)
	}
	client, err := factory()
	if err != nil {
		return true, err
	}
	return true, environmentClientCommand(ctx, client, args[1:], stdin, stdout, stderr)
}

func environmentClientCommand(
	ctx context.Context,
	client environmentControllerClient,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if client == nil || len(args) == 0 {
		return core.ErrInvalidArgument
	}
	switch args[0] {
	case "list":
		return environmentClientList(ctx, client, args[1:], stdout)
	case "create":
		return environmentClientCreate(ctx, client, args[1:], stdout)
	case "status":
		return environmentClientStatus(ctx, client, args[1:], stdout)
	case "exec":
		return environmentClientExec(ctx, client, args[1:], stdout, stderr)
	case "shell":
		return environmentClientShell(ctx, client, args[1:], stdin, stdout)
	case "delete":
		return environmentClientDelete(ctx, client, args[1:])
	default:
		return fmt.Errorf("unknown env command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func environmentClientList(ctx context.Context, client environmentControllerClient, args []string, out io.Writer) error {
	jsonOutput := false
	if len(args) == 1 && args[0] == "--json" {
		jsonOutput = true
	} else if len(args) != 0 {
		return fmt.Errorf("usage: haco env list [--json]: %w", core.ErrInvalidArgument)
	}
	environments, err := client.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	if environments == nil {
		environments = []core.Environment{}
	}
	if jsonOutput {
		return json.NewEncoder(out).Encode(environments)
	}
	for _, environment := range environments {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", environment.Name, environment.AccessMode, environment.Workspace.Path); err != nil {
			return err
		}
	}
	return nil
}

func environmentClientCreate(ctx context.Context, client environmentControllerClient, args []string, out io.Writer) error {
	request, err := parseEnvironmentClientCreateRequest(args)
	if err != nil {
		return err
	}
	environment, err := client.CreateEnvironment(ctx, request)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s\t%s\t%s\n", environment.Name, environment.Workspace.Path, environment.AccessMode)
	return err
}

func parseEnvironmentClientCreateRequest(args []string) (controlapi.EnvironmentCreateRequest, error) {
	usageError := func() error {
		return fmt.Errorf("usage: haco env create [--read-only] [--base <base>] [--cpu <n|unlimited>] [--memory <size|unlimited>] [--pids <n|unlimited>] [--root-size <size|unlimited>] --workspace <path> <environment>: %w", core.ErrInvalidArgument)
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
				return controlapi.EnvironmentCreateRequest{}, fmt.Errorf("unknown env create option %q: %w", args[0], core.ErrInvalidArgument)
			}
		}
	}
	if len(args) != 1 || strings.TrimSpace(request.WorkspacePath) == "" {
		return controlapi.EnvironmentCreateRequest{}, usageError()
	}
	request.Name = args[0]
	return request, nil
}

func environmentClientStatus(ctx context.Context, client environmentControllerClient, args []string, out io.Writer) error {
	if len(args) < 1 || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		return fmt.Errorf("usage: haco env status <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	status, err := client.EnvironmentStatus(ctx, args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		return json.NewEncoder(out).Encode(status)
	}
	if _, err := fmt.Fprintf(out, "name: %s\nstate: %s\nruntime: %s\nworkspace: %s\naccess: %s\n",
		status.Environment.Name, status.State, status.Environment.RuntimeRef,
		status.Environment.Workspace.Path, status.Environment.AccessMode); err != nil {
		return err
	}
	if status.Environment.Base != nil {
		if _, err := fmt.Fprintf(out, "base: %s\nbase-revision: %s\n", status.Environment.Base.Name, status.Environment.Base.Revision); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(out, "cpu: %s\nmemory-bytes: %s\npids: %s\nroot-bytes: %s\n",
		resourceLimitText(status.Environment.Resources.CPU),
		resourceLimitText(status.Environment.Resources.MemoryBytes),
		resourceLimitText(status.Environment.Resources.PIDs),
		resourceLimitText(status.Environment.Resources.RootBytes))
	return err
}

func environmentClientExec(ctx context.Context, client environmentControllerClient, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 3 || args[1] != "--" {
		return fmt.Errorf("usage: haco env exec <environment> -- <command...>: %w", core.ErrInvalidArgument)
	}
	result, err := client.ExecEnvironment(ctx, args[0], args[2:])
	if _, writeErr := io.WriteString(stdout, result.Stdout); writeErr != nil && err == nil {
		err = writeErr
	}
	if _, writeErr := io.WriteString(stderr, result.Stderr); writeErr != nil && err == nil {
		err = writeErr
	}
	return executionResultError(result, err)
}

func environmentClientShell(ctx context.Context, client environmentControllerClient, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco env shell <environment>: %w", core.ErrInvalidArgument)
	}
	stream, err := client.OpenEnvironmentShell(ctx, args[0])
	if err != nil {
		return err
	}
	defer stream.Close()

	inputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stream, stdin)
		if closer, ok := stream.(interface{ CloseWrite() error }); ok {
			if closeErr := closer.CloseWrite(); copyErr == nil {
				copyErr = closeErr
			}
		}
		inputDone <- copyErr
	}()

	_, outputErr := io.Copy(stdout, stream)
	_ = stream.Close()
	if outputErr != nil && !errors.Is(outputErr, net.ErrClosed) && ctx.Err() == nil {
		return outputErr
	}
	select {
	case inputErr := <-inputDone:
		if inputErr != nil && !errors.Is(inputErr, net.ErrClosed) && ctx.Err() == nil {
			return inputErr
		}
	default:
	}
	return ctx.Err()
}

func environmentClientDelete(ctx context.Context, client environmentControllerClient, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco env delete <environment>: %w", core.ErrInvalidArgument)
	}
	return client.DeleteEnvironment(ctx, args[0])
}
