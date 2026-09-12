package main

import (
	"context"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
)

func runCommand(ctx context.Context, app *composition.App, args []string) error {
	spec, jsonOutput, err := parseRunSpec(args)
	if err != nil {
		return err
	}
	result, runErr := app.Runner.Run(ctx, spec)
	return writeRunResult(os.Stdout, os.Stderr, result, jsonOutput, runErr)
}
