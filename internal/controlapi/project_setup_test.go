package controlapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/recipes"
	"github.com/SLktEx/Hacocoon/modules/standard/projectsetup"
)

type projectSetupFunc func(context.Context, string, recipes.Update) (projectsetup.Result, error)

func (f projectSetupFunc) Apply(ctx context.Context, n string, u recipes.Update) (projectsetup.Result, error) {
	return f(ctx, n, u)
}

func TestProjectSetupWireKeepsFailureAndRejectsUnexpectedOptions(t *testing.T) {
	calls := 0
	path := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterProjectSetup(server, projectSetupFunc(func(ctx context.Context, name string, update recipes.Update) (projectsetup.Result, error) {
			calls++
			if name != "dev" || update.Script == nil || *update.Script != "echo project" {
				t.Error("request changed")
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Error("missing deadline")
			}
			return projectsetup.Result{Environment: name, Applied: true, Execution: core.ExecutionResult{ExitCode: 17, Stdout: "result"}}, errors.New("private backend diagnostic")
		})); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	script := "echo project"
	response, err := client.SetupProject(context.Background(), "dev", recipes.Update{Script: &script})
	if err != nil || !response.Failed || response.Result.Execution.ExitCode != 17 || response.Result.Execution.Stdout != "result" || response.FailureCode != "internal" {
		t.Fatalf("failure lost: %#v %v", response, err)
	}
	wire, _ := control.NewClient(control.UnixDialer(path))
	for _, request := range []any{
		map[string]any{"environment": "dev", "source": "/host"},
		map[string]any{"environment": "dev", "update": map[string]any{"force": true}},
		map[string]any{"environment": "", "update": map[string]any{}},
		map[string]any{"environment": "dev", "update": map[string]any{"script": strings.Repeat("x", recipes.MaxScriptBytes+1)}},
	} {
		if err := wire.Call(context.Background(), MethodProjectSetup, request, nil); err == nil {
			t.Fatal("unexpected options accepted")
		}
	}
	if calls != 1 {
		t.Fatalf("service called %d times", calls)
	}
}
