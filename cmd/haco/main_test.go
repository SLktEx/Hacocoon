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
