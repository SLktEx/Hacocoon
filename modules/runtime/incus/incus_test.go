package incus

import (
	"context"
	"errors"
	"reflect"
	"strings"
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
	return host.Result{}, nil
}

func TestCreateEnvironmentUsesOnlyIncusWorkspaceLifecycle(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		if call == 0 {
			return host.Result{}, errors.New("project missing")
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)

	created, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/tmp/work space",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Ref != "haco-demo" {
		t.Fatalf("ref = %q", created.Ref)
	}

	want := []runnerCall{
		{name: "incus", args: []string{"project", "show", defaultProject}},
		{name: "incus", args: []string{"project", "create", defaultProject, "--config", "features.profiles=false"}},
		{name: "incus", args: []string{"init", defaultImage, "haco-demo", "--project", defaultProject}},
		{name: "incus", args: []string{"config", "device", "add", "haco-demo", "workspace", "disk", "source=/tmp/work space", "path=/workspace", "--project", defaultProject}},
		{name: "incus", args: []string{"start", "haco-demo", "--project", defaultProject}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v\nwant = %#v", runner.calls, want)
	}
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		for _, forbidden := range []string{"storage create", "qcow", "btrfs", "security.nesting"} {
			if strings.Contains(joined, forbidden) {
				t.Fatalf("v0.1 environment path leaked %q into %q", forbidden, joined)
			}
		}
	}
}

func TestCreateEnvironmentReusesExistingProject(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	_, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace"})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 2 && call.args[0] == "project" && call.args[1] == "create" {
			t.Fatalf("unexpected project create: %#v", call)
		}
	}
}

func TestCreateEnvironmentCleansUpAfterWorkspaceMountFailure(t *testing.T) {
	mountErr := errors.New("mount denied")
	runner := &fakeRunner{run: func(ctx context.Context, call int, _ string, _ []string) (host.Result, error) {
		switch call {
		case 2:
			return host.Result{}, mountErr
		case 3:
			if ctx.Err() != nil {
				t.Fatalf("cleanup context is canceled: %v", ctx.Err())
			}
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runtime.CreateEnvironment(ctx, core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace"})
	if !errors.Is(err, mountErr) {
		t.Fatalf("error = %v", err)
	}
	last := runner.calls[len(runner.calls)-1]
	assertRunnerCall(t, last, "incus", "delete", "haco-demo", "--project", defaultProject, "--force")
}

func TestCreateEnvironmentCleansUpAfterStartFailure(t *testing.T) {
	startErr := errors.New("start failed")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		if call == 3 {
			return host.Result{}, startErr
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)

	_, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace"})
	if !errors.Is(err, startErr) {
		t.Fatalf("error = %v", err)
	}
	last := runner.calls[len(runner.calls)-1]
	assertRunnerCall(t, last, "incus", "delete", "haco-demo", "--project", defaultProject, "--force")
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

func TestDeleteEnvironmentIsProjectScopedAndForced(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	if err := runtime.DeleteEnvironment(context.Background(), "haco-demo"); err != nil {
		t.Fatal(err)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "delete", "haco-demo", "--project", defaultProject, "--force")
}

func TestProbeReportsIncusAvailability(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{Stdout: "6.12\n"}, nil
	}}
	caps, err := New(runner).Probe(context.Background())
	if err != nil || !caps.Available || !reflect.DeepEqual(caps.Details, []string{"6.12"}) {
		t.Fatalf("caps=%#v err=%v", caps, err)
	}
}

func assertRunnerCall(t *testing.T, call runnerCall, name string, args ...string) {
	t.Helper()
	if call.name != name || !reflect.DeepEqual(call.args, args) {
		t.Fatalf("call = %#v, want name=%q args=%#v", call, name, args)
	}
}
