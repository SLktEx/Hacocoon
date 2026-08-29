package main

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestDispatchRejectsUnknownCommand(t *testing.T) {
	err := dispatch(context.Background(), &composition.App{}, []string{"wat"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("dispatch error = %v", err)
	}
}

func TestCommandExitErrorCarriesExitCode(t *testing.T) {
	err := commandExitError{code: 23}
	if err.ExitCode() != 23 {
		t.Fatalf("exit code = %d", err.ExitCode())
	}
}

func TestExecutionResultErrorPreservesInfrastructureFailure(t *testing.T) {
	infraErr := errors.New("incus daemon unavailable")
	err := executionResultError(core.ExecutionResult{ExitCode: 1}, infraErr)
	if !errors.Is(err, infraErr) {
		t.Fatalf("infrastructure error was hidden: %v", err)
	}
}

func TestExecutionResultErrorUsesRemoteExitWithoutInfrastructureFailure(t *testing.T) {
	err := executionResultError(core.ExecutionResult{ExitCode: 7}, nil)
	var exitErr commandExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("remote exit error = %T %v", err, err)
	}
}
