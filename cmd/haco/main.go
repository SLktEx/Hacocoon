package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
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
		"create": createCommand,
		"exec":   execCommand,
		"shell":  shellCommand,
		"delete": deleteCommand,
		"doctor": doctorCommand,
	}
	run, ok := commands[args[0]]
	if !ok {
		usage()
		return fmt.Errorf("unknown command %q: %w", args[0], core.ErrInvalidArgument)
	}
	return run(ctx, app, args[1:])
}

func createCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 3 || args[0] != "--workspace" {
		return fmt.Errorf("usage: haco create --workspace <path> <environment>: %w", core.ErrInvalidArgument)
	}
	environment, err := app.Environments.Create(ctx, core.EnvironmentSpec{
		WorkspacePath: args[1],
		Name:          args[2],
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s\t%s\n", environment.Name, environment.Workspace.Path)
	return nil
}

func execCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 3 || args[1] != "--" {
		return fmt.Errorf("usage: haco exec <environment> -- <command...>: %w", core.ErrInvalidArgument)
	}
	result, err := app.Environments.Exec(ctx, args[0], core.ExecutionRequest{Argv: args[2:]})
	fmt.Print(result.Stdout)
	fmt.Fprint(os.Stderr, result.Stderr)
	if result.ExitCode > 0 {
		return commandExitError{code: result.ExitCode}
	}
	if err != nil {
		return err
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
	fmt.Fprintln(os.Stderr, "usage: haco <create|exec|shell|delete|doctor>")
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
