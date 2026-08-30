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

func TestParseCreateSpecAcceptsExplicitBase(t *testing.T) {
	spec, err := parseCreateSpec([]string{"--base", "my-dev", "--workspace", "/tmp/workspace", "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "demo" || spec.WorkspacePath != "/tmp/workspace" || spec.Base != "my-dev" || spec.AccessMode != core.WorkspaceReadWrite {
		t.Fatalf("spec=%#v", spec)
	}
}

func TestParseCreateSpecRejectsDuplicateBase(t *testing.T) {
	_, err := parseCreateSpec([]string{"--base", "a", "--base", "b", "--workspace", "/tmp/workspace", "demo"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseCreateSpecCombinesReadOnlyAndBase(t *testing.T) {
	spec, err := parseCreateSpec([]string{"--read-only", "--workspace", "/tmp/workspace", "--base", "haco/ubuntu-24.04", "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.AccessMode != core.WorkspaceReadOnly || spec.Base != "haco/ubuntu-24.04" {
		t.Fatalf("spec=%#v", spec)
	}
}
