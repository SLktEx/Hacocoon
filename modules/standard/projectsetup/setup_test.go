package projectsetup

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

type fixture struct {
	environment core.Environment
	starts      int
	requests    []core.ExecutionRequest
	fail        error
}

func (f *fixture) Get(context.Context, string) (core.Environment, error) { return f.environment, nil }
func (f *fixture) StartForWorkspace(_ context.Context, _ string, id core.WorkspaceID) error {
	if id != f.environment.Workspace.ID {
		return core.ErrIncompatibleState
	}
	f.starts++
	return f.fail
}
func (f *fixture) ExecForWorkspace(_ context.Context, _ string, id core.WorkspaceID, r core.ExecutionRequest) (core.ExecutionResult, error) {
	if id != f.environment.Workspace.ID {
		return core.ExecutionResult{}, core.ErrIncompatibleState
	}
	f.requests = append(f.requests, r)
	return core.ExecutionResult{ExitCode: 17, Stdout: "project output"}, nil
}

func TestWorkspaceRecipeSaveReplayIsolationAndClear(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux private recipe store")
	}
	f := &fixture{environment: core.Environment{Name: "dev", Workspace: core.Workspace{ID: "one", Path: "/project"}}}
	s := Service{Root: t.TempDir(), Environments: f}
	script := "printf project"
	result, err := s.Apply(context.Background(), "dev", recipes.Update{Script: &script})
	if !errors.Is(err, recipes.ErrExecutionFailed) || !result.Applied || result.Execution.ExitCode != 17 || result.FailureStage != "script" {
		t.Fatalf("nonzero result lost: %#v %v", result, err)
	}
	if len(f.requests) != 1 || string(f.requests[0].Stdin) != script || strings.Contains(strings.Join(f.requests[0].Argv, " "), script) {
		t.Fatal("script must travel only through stdin")
	}
	if f.requests[0].WorkingDirectory != "/workspace" {
		t.Fatal("wrong working directory")
	}
	// A second Environment for the same Workspace reuses the saved snapshot.
	f.environment.Name = "second"
	_, _ = s.Apply(context.Background(), "second", recipes.Update{})
	if len(f.requests) != 2 {
		t.Fatal("snapshot was not retained after failure")
	}
	f.environment.Workspace.ID = "two"
	result, err = s.Apply(context.Background(), "second", recipes.Update{})
	if err != nil || result.Applied || len(f.requests) != 2 {
		t.Fatal("recipe crossed Workspace identity")
	}
	f.environment.Workspace.ID = "one"
	result, err = s.Apply(context.Background(), "second", recipes.Update{Clear: true})
	if err != nil || !result.Cleared || result.Applied {
		t.Fatal("clear executed recipe")
	}
	_, err = s.Apply(context.Background(), "second", recipes.Update{})
	if err != nil || len(f.requests) != 2 {
		t.Fatal("cleared recipe replayed")
	}
}

func TestProjectSetupRefusesStartFailureBeforeExecution(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux private recipe store")
	}
	f := &fixture{environment: core.Environment{Name: "dev", Workspace: core.Workspace{ID: "one", Path: "/project"}}, fail: core.ErrIncompatibleState}
	s := Service{Root: t.TempDir(), Environments: f}
	script := "echo secret"
	result, err := s.Apply(context.Background(), "dev", recipes.Update{Script: &script})
	if !errors.Is(err, core.ErrIncompatibleState) || result.Applied || len(f.requests) != 0 || result.FailureStage != "start" {
		t.Fatal("executed after failed identity-bound start")
	}
}
