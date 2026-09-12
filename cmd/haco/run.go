package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
)

func runCommand(ctx context.Context, app *composition.App, args []string) error {
	spec, jsonOutput, err := parseRunSpec(args)
	if err != nil {
		return err
	}
	result, runErr := app.Runner.Run(ctx, spec)
	if jsonOutput {
		payload, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
	} else {
		fmt.Print(result.Execution.Stdout)
		fmt.Fprint(os.Stderr, result.Execution.Stderr)
	}
	if runErr != nil {
		return runErr
	}
	if result.Execution.ExitCode > 0 {
		return commandExitError{code: result.Execution.ExitCode}
	}
	return nil
}
