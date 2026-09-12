package main

import (
	"context"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

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

func execCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 3 || args[1] != "--" {
		return fmt.Errorf("usage: haco exec <environment> -- <command...>: %w", core.ErrInvalidArgument)
	}
	result, err := app.Environments.Exec(ctx, args[0], core.ExecutionRequest{Argv: args[2:]})
	fmt.Print(result.Stdout)
	fmt.Fprint(os.Stderr, result.Stderr)
	return executionResultError(result, err)
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
