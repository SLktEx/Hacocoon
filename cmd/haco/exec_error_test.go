package main

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

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
