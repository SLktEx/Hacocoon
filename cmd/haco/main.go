package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	gitcapapp "github.com/SLktEx/Hacocoon/internal/gitcap"
)

type command func(context.Context, *composition.App, []string) error

type commandExitError struct {
	code int
}

func (e commandExitError) Error() string { return fmt.Sprintf("command exited %d", e.code) }
func (e commandExitError) ExitCode() int { return e.code }

func main() {
	ctx := context.Background()
	app, err := composition.Local(ctx)
	if err != nil {
		fail(err)
	}
	if err := dispatch(ctx, app, os.Args[1:]); err != nil {
		fail(err)
	}
}

func dispatch(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		usage()
		return core.ErrInvalidArgument
	}
	commands := map[string]command{
		"create":      createCommand,
		"git":         gitCommand,
		"capability":  capabilityCommand,
		"status":      statusCommand,
		"connections": connectionsCommand,
		"forward":     forwardCommand,
		"unforward":   unforwardCommand,
		"ssh":         sshCommand,
		"exec":        execCommand,
		"shell":       shellCommand,
		"delete":      deleteCommand,
		"doctor":      doctorCommand,
	}
	run, ok := commands[args[0]]
	if !ok {
		usage()
		return fmt.Errorf("unknown command %q: %w", args[0], core.ErrInvalidArgument)
	}
	return run(ctx, app, args[1:])
}

func createCommand(ctx context.Context, app *composition.App, args []string) error {
	spec, err := parseCreateSpec(args)
	if err != nil {
		return err
	}
	environment, err := app.Environments.Create(ctx, spec)
	if err != nil {
		return err
	}
	fmt.Printf("%s\t%s\t%s\n", environment.Name, environment.Workspace.Path, environment.AccessMode)
	return nil
}

func parseCreateSpec(args []string) (core.EnvironmentSpec, error) {
	if len(args) < 3 {
		return core.EnvironmentSpec{}, fmt.Errorf("usage: haco create [--read-only] --workspace <path> <environment>: %w", core.ErrInvalidArgument)
	}
	spec := core.EnvironmentSpec{AccessMode: core.WorkspaceReadWrite}
	for len(args) > 1 {
		switch args[0] {
		case "--read-only":
			spec.AccessMode = core.WorkspaceReadOnly
			args = args[1:]
		case "--workspace":
			if len(args) < 3 || spec.WorkspacePath != "" {
				return core.EnvironmentSpec{}, fmt.Errorf("usage: haco create [--read-only] --workspace <path> <environment>: %w", core.ErrInvalidArgument)
			}
			spec.WorkspacePath = args[1]
			args = args[2:]
		default:
			if len(args) != 1 {
				return core.EnvironmentSpec{}, fmt.Errorf("unknown create option %q: %w", args[0], core.ErrInvalidArgument)
			}
		}
	}
	if len(args) != 1 || spec.WorkspacePath == "" {
		return core.EnvironmentSpec{}, fmt.Errorf("usage: haco create [--read-only] --workspace <path> <environment>: %w", core.ErrInvalidArgument)
	}
	spec.Name = args[0]
	return spec, nil
}

func gitCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 2 || args[0] != "push" {
		return fmt.Errorf("usage: haco git push <environment> --branch <branch> [--source <revision>] [--remote <remote>] [--force]: %w", core.ErrInvalidArgument)
	}
	spec, err := parseGitPushSpec(args[1:])
	if err != nil {
		return err
	}
	result, err := app.Git.Push(ctx, spec)
	if result.Output != "" {
		fmt.Println(result.Output)
	}
	return err
}

func parseGitPushSpec(args []string) (gitcapapp.PushSpec, error) {
	if len(args) < 3 {
		return gitcapapp.PushSpec{}, core.ErrInvalidArgument
	}
	spec := gitcapapp.PushSpec{Environment: args[0], Remote: "origin", Source: "HEAD"}
	args = args[1:]
	seenBranch := false
	seenSource := false
	seenRemote := false
	for len(args) > 0 {
		switch args[0] {
		case "--branch":
			if len(args) < 2 || seenBranch {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Branch, seenBranch, args = args[1], true, args[2:]
		case "--source":
			if len(args) < 2 || seenSource {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Source, seenSource, args = args[1], true, args[2:]
		case "--remote":
			if len(args) < 2 || seenRemote {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Remote, seenRemote, args = args[1], true, args[2:]
		case "--force":
			if spec.Force {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Force, args = true, args[1:]
		default:
			return gitcapapp.PushSpec{}, fmt.Errorf("unknown git push option %q: %w", args[0], core.ErrInvalidArgument)
		}
	}
	if strings.TrimSpace(spec.Environment) == "" || !seenBranch || strings.TrimSpace(spec.Branch) == "" {
		return gitcapapp.PushSpec{}, core.ErrInvalidArgument
	}
	return spec, nil
}

func capabilityCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 3 || args[0] != "request" {
		return fmt.Errorf("usage: haco capability request <capability> <action> [--resource <resource>] [--environment <environment>] [--param <key=value>]...: %w", core.ErrInvalidArgument)
	}
	req, err := parseCapabilityRequest(args[1:])
	if err != nil {
		return err
	}
	result, err := app.Capabilities.Request(ctx, req)
	if result.Output != "" {
		fmt.Println(result.Output)
	}
	return err
}

func parseCapabilityRequest(args []string) (core.CapabilityRequest, error) {
	if len(args) < 2 {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	req := core.CapabilityRequest{Capability: args[0], Action: args[1], Parameters: map[string]string{}}
	args = args[2:]
	for len(args) > 0 {
		switch args[0] {
		case "--resource":
			if len(args) < 2 || req.Resource != "" {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			req.Resource = args[1]
			args = args[2:]
		case "--environment":
			if len(args) < 2 || req.Environment != "" {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			req.Environment = args[1]
			args = args[2:]
		case "--param":
			if len(args) < 2 {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			parts := strings.SplitN(args[1], "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			key := strings.TrimSpace(parts[0])
			if _, exists := req.Parameters[key]; exists {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			req.Parameters[key] = parts[1]
			args = args[2:]
		default:
			return core.CapabilityRequest{}, fmt.Errorf("unknown capability option %q: %w", args[0], core.ErrInvalidArgument)
		}
	}
	return req, nil
}

func statusCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 1 || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		return fmt.Errorf("usage: haco status <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	status, err := app.Clients.Status(ctx, args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		payload, err := json.Marshal(status)
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	fmt.Printf("name: %s\nstate: %s\nruntime: %s\nworkspace: %s\naccess: %s\n",
		status.Environment.Name, status.State, status.Environment.RuntimeRef,
		status.Environment.Workspace.Path, status.Environment.AccessMode)
	return nil
}

func connectionsCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 1 || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		return fmt.Errorf("usage: haco connections <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	connections, err := app.Clients.Connections(ctx, args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		payload, err := json.Marshal(connections)
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	for _, connection := range connections {
		fmt.Printf("%s\t%s\t%s:%d\t->\t%d\n", connection.ID, connection.Kind, connection.Host, connection.Port, connection.TargetPort)
	}
	return nil
}

func forwardCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 5 || args[1] != "--host-port" || args[3] != "--target-port" {
		return fmt.Errorf("usage: haco forward <environment> --host-port <port> --target-port <port>: %w", core.ErrInvalidArgument)
	}
	hostPort, err := parsePort(args[2])
	if err != nil {
		return err
	}
	targetPort, err := parsePort(args[4])
	if err != nil {
		return err
	}
	connection, err := app.Clients.Forward(ctx, args[0], core.LocalPortRequest{Protocol: "tcp", HostPort: hostPort, TargetPort: targetPort})
	if err != nil {
		return err
	}
	fmt.Printf("%s\ttcp://%s:%d\t->\t%d\n", connection.ID, connection.Host, connection.Port, connection.TargetPort)
	return nil
}

func unforwardCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: haco unforward <environment> <connection-id>: %w", core.ErrInvalidArgument)
	}
	return app.Clients.Unforward(ctx, args[0], args[1])
}

func sshCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 5 || args[1] != "--public-key" || args[3] != "--host-port" {
		return fmt.Errorf("usage: haco ssh <environment> --public-key <path> --host-port <port>: %w", core.ErrInvalidArgument)
	}
	key, err := os.ReadFile(args[2])
	if err != nil {
		return fmt.Errorf("read SSH public key: %w", err)
	}
	hostPort, err := parsePort(args[4])
	if err != nil {
		return err
	}
	connection, err := app.Clients.SSH(ctx, args[0], core.SSHAccessRequest{PublicKey: string(key), HostPort: hostPort})
	if err != nil {
		return err
	}
	fmt.Println(connection.Command)
	return nil
}

func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %q: %w", raw, core.ErrInvalidArgument)
	}
	return port, nil
}

func execCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 3 || args[1] != "--" {
		return fmt.Errorf("usage: haco exec <environment> -- <command...>: %w", core.ErrInvalidArgument)
	}
	result, err := app.Environments.Exec(ctx, args[0], core.ExecutionRequest{Argv: args[2:]})
	fmt.Print(result.Stdout)
	fmt.Fprint(os.Stderr, result.Stderr)
	return executionResultError(result, err)
}

func executionResultError(result core.ExecutionResult, err error) error {
	if err != nil {
		return err
	}
	if result.ExitCode > 0 {
		return commandExitError{code: result.ExitCode}
	}
	return nil
}

func shellCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco shell <environment>: %w", core.ErrInvalidArgument)
	}
	return app.Environments.Shell(ctx, args[0])
}

func deleteCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco delete <environment>: %w", core.ErrInvalidArgument)
	}
	return app.Environments.Delete(ctx, args[0])
}

func doctorCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: haco doctor: %w", core.ErrInvalidArgument)
	}
	caps, err := app.Runtime.Probe(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Hacocoon Secure Workspace Runtime")
	fmt.Printf("Incus available: %t\n", caps.Available)
	for _, detail := range caps.Details {
		fmt.Printf("  %s\n", detail)
	}
	if !caps.Available {
		return core.ErrRuntimeUnavailable
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: haco <create|git|status|connections|forward|unforward|ssh|capability|exec|shell|delete|doctor>")
}

func fail(err error) {
	code := 1
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) && exitCoder.ExitCode() > 0 {
		code = exitCoder.ExitCode()
	}
	message := strings.TrimSpace(err.Error())
	if message != "" {
		fmt.Fprintln(os.Stderr, "haco:", message)
	}
	os.Exit(code)
}
