package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls []runnerCall
	run   func(context.Context, int, string, []string) (host.Result, error)
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	copyArgs := append([]string(nil), args...)
	f.calls = append(f.calls, runnerCall{name: name, args: copyArgs})
	if f.run != nil {
		return f.run(ctx, len(f.calls)-1, name, copyArgs)
	}
	if len(args) >= 2 && args[0] == "profile" && args[1] == "show" {
		return rootProfileResult(), nil
	}
	return host.Result{}, nil
}

func rootProfileResult() host.Result {
	return host.Result{Stdout: `{"devices":{"root":{"type":"disk","path":"/","pool":"default"}}}`}
}

func testSandboxProvider(t *testing.T, runtime *Runtime) *SandboxProvider {
	t.Helper()
	p, err := NewSandboxProvider(runtime)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Seed an already selected pool in low-level fixtures. Product composition uses
// ConfigureStorageProvider and resolves its owned pool lazily.
func (r *Runtime) setRootPool(pool string) {
	if r.storage == nil {
		r.storage = &runtimeStorageState{}
	}
	r.storage.mu.Lock()
	defer r.storage.mu.Unlock()
	r.storage.rootPool = pool
}

func TestExecEnvironmentPreservesArgumentBoundariesAndResult(t *testing.T) {
	exitErr := errors.New("exit 17")
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{ExitCode: 17, Stdout: "stdout", Stderr: "stderr"}, exitErr
	}}
	runtime := New(runner)

	result, err := runtime.ExecEnvironment(context.Background(), "haco-demo", core.ExecutionRequest{Argv: []string{"printf", "%s", "hello world"}})
	if !errors.Is(err, exitErr) || result.ExitCode != 17 || result.Stdout != "stdout" || result.Stderr != "stderr" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "exec", "haco-demo", "--project", defaultProject, "--", "printf", "%s", "hello world")
}

func TestDeleteEnvironmentTreatsConfirmedAbsenceAsCoreNotFound(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		if call == 0 {
			return host.Result{ExitCode: 1, Stderr: "Error: Instance not found"}, errors.New("exit status 1")
		}
		return host.Result{Stdout: ""}, nil
	}}
	err := New(runner).DeleteEnvironment(context.Background(), "haco-demo")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	assertRunnerCall(t, runner.calls[1], "incus", "list", "haco-demo", "--project", defaultProject, "--format", "csv", "-c", "n")
}

func TestDeleteEnvironmentDoesNotTrustUnrelatedNotFoundText(t *testing.T) {
	deleteErr := errors.New("exit status 1")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		if call == 0 {
			return host.Result{ExitCode: 1, Stderr: "Error: dependent network not found"}, deleteErr
		}
		return host.Result{Stdout: "haco-demo\n"}, nil
	}}
	err := New(runner).DeleteEnvironment(context.Background(), "haco-demo")
	if !errors.Is(err, deleteErr) || errors.Is(err, core.ErrNotFound) {
		t.Fatalf("error = %v, want original delete failure only", err)
	}
}

func TestDeleteEnvironmentIsProjectScopedAndForced(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	if err := runtime.DeleteEnvironment(context.Background(), "haco-demo"); err != nil {
		t.Fatal(err)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "delete", "haco-demo", "--project", defaultProject, "--force")
}

func assertRunnerCall(t *testing.T, call runnerCall, name string, args ...string) {
	t.Helper()
	if call.name != name || !reflect.DeepEqual(call.args, args) {
		t.Fatalf("call = %#v, want name=%q args=%#v", call, name, args)
	}
}
