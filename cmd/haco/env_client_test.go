package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeEnvironmentController struct {
	createdRequest controlapi.EnvironmentCreateRequest
	created        core.Environment
	listed         []core.Environment
	status         core.EnvironmentStatus
	execResult     core.ExecutionResult
	execErr        error
	deleted        string
}

func (f *fakeEnvironmentController) CreateEnvironment(_ context.Context, request controlapi.EnvironmentCreateRequest) (core.Environment, error) {
	f.createdRequest = request
	return f.created, nil
}

func (f *fakeEnvironmentController) ListEnvironments(context.Context) ([]core.Environment, error) {
	return append([]core.Environment(nil), f.listed...), nil
}

func (f *fakeEnvironmentController) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return f.status, nil
}

func (f *fakeEnvironmentController) ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error) {
	return f.execResult, f.execErr
}

func (f *fakeEnvironmentController) OpenEnvironmentShell(context.Context, string) (net.Conn, error) {
	return nil, errors.New("shell not configured")
}

func (f *fakeEnvironmentController) DeleteEnvironment(_ context.Context, environment string) error {
	f.deleted = environment
	return nil
}

func TestHandleEnvironmentClientArgsIgnoresNonEnvironmentCommands(t *testing.T) {
	factoryCalls := 0
	handled, err := handleEnvironmentClientArgs(context.Background(), []string{"doctor"}, func() (environmentControllerClient, error) {
		factoryCalls++
		return &fakeEnvironmentController{}, nil
	}, bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	if err != nil {
		t.Fatal(err)
	}
	if handled {
		t.Fatal("non-environment command was intercepted")
	}
	if factoryCalls != 0 {
		t.Fatalf("controller factory calls = %d, want 0", factoryCalls)
	}
}

func TestHandleEnvironmentClientArgsWorksWithoutLocalComposition(t *testing.T) {
	client := &fakeEnvironmentController{}
	var out bytes.Buffer
	factoryCalls := 0
	handled, err := handleEnvironmentClientArgs(context.Background(), []string{"env", "list", "--json"}, func() (environmentControllerClient, error) {
		factoryCalls++
		return client, nil
	}, bytes.NewBuffer(nil), &out, bytes.NewBuffer(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("haco env command was not intercepted before local composition")
	}
	if factoryCalls != 1 {
		t.Fatalf("controller factory calls = %d, want 1", factoryCalls)
	}
	if out.String() != "[]\n" {
		t.Fatalf("output = %q, want empty JSON list", out.String())
	}
}

func TestEnvironmentClientCreatePreservesRequestContract(t *testing.T) {
	client := &fakeEnvironmentController{created: core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{Path: "/work/demo"},
		AccessMode: core.WorkspaceReadOnly,
	}}
	var out bytes.Buffer
	err := environmentClientCreate(context.Background(), client, []string{
		"--read-only",
		"--base", "haco/ubuntu-26.04",
		"--cpu", "2",
		"--memory", "1GiB",
		"--pids", "128",
		"--root-size", "8GiB",
		"--workspace", "/work/demo",
		"demo",
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	request := client.createdRequest
	if request.Name != "demo" || request.WorkspacePath != "/work/demo" || request.AccessMode != core.WorkspaceReadOnly {
		t.Fatalf("request identity = %+v", request)
	}
	if request.Base != core.BaseName("haco/ubuntu-26.04") {
		t.Fatalf("base = %q", request.Base)
	}
	if request.Resources.CPU.Mode != core.ResourceLimitFinite || request.Resources.CPU.Value != 2 {
		t.Fatalf("cpu = %+v", request.Resources.CPU)
	}
	if request.Resources.MemoryBytes.Mode != core.ResourceLimitFinite || request.Resources.MemoryBytes.Value != 1024*1024*1024 {
		t.Fatalf("memory = %+v", request.Resources.MemoryBytes)
	}
	if request.Resources.PIDs.Mode != core.ResourceLimitFinite || request.Resources.PIDs.Value != 128 {
		t.Fatalf("pids = %+v", request.Resources.PIDs)
	}
	if request.Resources.RootBytes.Mode != core.ResourceLimitFinite || request.Resources.RootBytes.Value != 8*1024*1024*1024 {
		t.Fatalf("root = %+v", request.Resources.RootBytes)
	}
	if out.String() != "demo\t/work/demo\tro\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestEnvironmentClientExecPreservesGuestExitStatus(t *testing.T) {
	client := &fakeEnvironmentController{execResult: core.ExecutionResult{
		ExitCode: 23,
		Stdout:   "out\n",
		Stderr:   "err\n",
	}}
	var stdout, stderr bytes.Buffer
	err := environmentClientExec(context.Background(), client, []string{"demo", "--", "false"}, &stdout, &stderr)
	var exitErr commandExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
		t.Fatalf("error = %v, want exit code 23", err)
	}
	if stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestEnvironmentClientDeleteUsesController(t *testing.T) {
	client := &fakeEnvironmentController{}
	if err := environmentClientDelete(context.Background(), client, []string{"demo"}); err != nil {
		t.Fatal(err)
	}
	if client.deleted != "demo" {
		t.Fatalf("deleted = %q", client.deleted)
	}
}
