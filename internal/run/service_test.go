package run

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeEnvironments struct {
	calls      []string
	createSpec core.EnvironmentSpec
	execReq    core.ExecutionRequest
	createErr  error
	execResult core.ExecutionResult
	execErr    error
	deleteErr  error
}

func (f *fakeEnvironments) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	f.calls = append(f.calls, "create")
	f.createSpec = spec
	if f.createErr != nil {
		return core.Environment{}, f.createErr
	}
	return core.Environment{Name: spec.Name}, nil
}
func (f *fakeEnvironments) Exec(_ context.Context, name string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	f.calls = append(f.calls, "exec:"+name)
	f.execReq = req
	return f.execResult, f.execErr
}
func (f *fakeEnvironments) Delete(_ context.Context, name string) error {
	f.calls = append(f.calls, "delete:"+name)
	return f.deleteErr
}

func serviceWithName(env *fakeEnvironments, name string) *Service {
	s := New(env)
	s.newName = func() (string, error) { return name, nil }
	return s
}

func TestRunComposesCreateExecCleanup(t *testing.T) {
	env := &fakeEnvironments{execResult: core.ExecutionResult{Stdout: "ok\n", ExitCode: 0}}
	service := serviceWithName(env, "run-abc")
	result, err := service.Run(context.Background(), Spec{
		WorkspacePath: "/work/demo",
		AccessMode:    core.WorkspaceReadOnly,
		Argv:          []string{"sh", "-c", "printf ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(env.calls, []string{"create", "exec:run-abc", "delete:run-abc"}) {
		t.Fatalf("calls=%v", env.calls)
	}
	if env.createSpec.WorkspacePath != "/work/demo" || env.createSpec.AccessMode != core.WorkspaceReadOnly {
		t.Fatalf("create=%#v", env.createSpec)
	}
	if !reflect.DeepEqual(env.execReq.Argv, []string{"sh", "-c", "printf ok"}) {
		t.Fatalf("argv=%v", env.execReq.Argv)
	}
	if result.Environment != "run-abc" || !result.CleanedUp || result.Execution.Stdout != "ok\n" {
		t.Fatalf("result=%#v", result)
	}
}

func TestRunCleansUpAfterExecutionFailure(t *testing.T) {
	execErr := errors.New("execution failed")
	env := &fakeEnvironments{execResult: core.ExecutionResult{ExitCode: 17}, execErr: execErr}
	result, err := serviceWithName(env, "run-fail").Run(context.Background(), Spec{WorkspacePath: "/work/demo", Argv: []string{"false"}})
	if !errors.Is(err, execErr) || !result.CleanedUp || result.Execution.ExitCode != 17 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !reflect.DeepEqual(env.calls, []string{"create", "exec:run-fail", "delete:run-fail"}) {
		t.Fatalf("calls=%v", env.calls)
	}
}

func TestRunSurfacesCleanupFailure(t *testing.T) {
	cleanupErr := errors.New("delete failed")
	env := &fakeEnvironments{deleteErr: cleanupErr}
	result, err := serviceWithName(env, "run-cleanup").Run(context.Background(), Spec{WorkspacePath: "/work/demo", Argv: []string{"true"}})
	if !errors.Is(err, cleanupErr) || result.CleanedUp {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestRunJoinsExecutionAndCleanupFailures(t *testing.T) {
	execErr := errors.New("exec failed")
	cleanupErr := errors.New("cleanup failed")
	env := &fakeEnvironments{execErr: execErr, deleteErr: cleanupErr}
	_, err := serviceWithName(env, "run-both").Run(context.Background(), Spec{WorkspacePath: "/work/demo", Argv: []string{"false"}})
	if !errors.Is(err, execErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("err=%v", err)
	}
}

func TestRunDoesNotDeleteWhenCreateFails(t *testing.T) {
	createErr := errors.New("create failed")
	env := &fakeEnvironments{createErr: createErr}
	_, err := serviceWithName(env, "run-create").Run(context.Background(), Spec{WorkspacePath: "/work/demo", Argv: []string{"true"}})
	if !errors.Is(err, createErr) || !reflect.DeepEqual(env.calls, []string{"create"}) {
		t.Fatalf("calls=%v err=%v", env.calls, err)
	}
}
