package incus

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestCreateEnvironmentUsesIsolatedProfileAndShiftedWritableWorkspace(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
		if call == 0 {
			return host.Result{}, errors.New("project missing")
		}
		if len(args) >= 2 && args[0] == "profile" && args[1] == "show" {
			return rootProfileResult(), nil
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
		{name: "incus", args: []string{"profile", "show", "default", "--project", "default", "--format", "json"}},
		{name: "incus", args: []string{"init", defaultImage, "haco-demo", "--project", defaultProject, "--profile", sandboxProfile, "--storage", "default"}},
		{name: "incus", args: []string{"config", "device", "override", "haco-demo", "eth0", "name=eth0", "network=" + sandboxNetwork, "security.ipv4_filtering=true", "security.ipv6_filtering=true", "security.mac_filtering=true", "security.port_isolation=true", "--project", defaultProject}},
		{name: "incus", args: []string{"config", "device", "add", "haco-demo", "workspace", "disk", "source=/tmp/work space", "path=/workspace", "shift=true", "--project", defaultProject}},
		{name: "incus", args: []string{"start", "haco-demo", "--project", defaultProject}},
		{name: "incus", args: []string{"exec", "haco-demo", "--project", defaultProject, "--", "test", "-w", "/workspace"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v\nwant = %#v", runner.calls, want)
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

func TestCreateEnvironmentReadOnlyDoesNotRequestShiftOrWriteProbe(t *testing.T) {
	runner := &fakeRunner{}
	_, err := New(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	seenReadonly := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "readonly=true") {
			seenReadonly = true
		}
		if strings.Contains(joined, "shift=true") {
			t.Fatalf("read-only mount requested id shifting: %#v", call)
		}
		if strings.Contains(joined, " test -w /workspace") {
			t.Fatalf("read-only mount was write-probed: %#v", call)
		}
	}
	if !seenReadonly {
		t.Fatalf("readonly=true missing from calls: %#v", runner.calls)
	}
}

func TestCreateEnvironmentCleansUpAfterNICMaterializationFailure(t *testing.T) {
	nicErr := errors.New("NIC override failed")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "profile" && args[1] == "show" {
			return rootProfileResult(), nil
		}
		if call == 3 {
			return host.Result{Stderr: "bad inherited device"}, nicErr
		}
		return host.Result{}, nil
	}}
	_, err := New(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace"})
	if !errors.Is(err, nicErr) || !strings.Contains(err.Error(), "bad inherited device") {
		t.Fatalf("error = %v", err)
	}
	last := runner.calls[len(runner.calls)-1]
	assertRunnerCall(t, last, "incus", "delete", "haco-demo", "--project", defaultProject, "--force")
}

func TestCreateEnvironmentCleansUpAfterWorkspaceMountFailureWithDetachedContext(t *testing.T) {
	mountErr := errors.New("mount denied")
	runner := &fakeRunner{run: func(ctx context.Context, call int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "profile" && args[1] == "show" {
			return rootProfileResult(), nil
		}
		if call == 4 {
			return host.Result{}, mountErr
		}
		if call == 5 && ctx.Err() != nil {
			t.Fatalf("cleanup context is canceled: %v", ctx.Err())
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

func TestCreateEnvironmentCleanupIsBoundedAndSignalsRecovery(t *testing.T) {
	mountErr := errors.New("mount denied")
	runner := &fakeRunner{run: func(ctx context.Context, call int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "profile" && args[1] == "show" {
			return rootProfileResult(), nil
		}
		if call == 4 {
			return host.Result{}, mountErr
		}
		if call == 5 {
			<-ctx.Done()
			return host.Result{}, ctx.Err()
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	runtime.cleanupTimeout = 20 * time.Millisecond

	started := time.Now()
	_, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace"})
	if !errors.Is(err, mountErr) || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup exceeded bounded deadline: %v", elapsed)
	}
}

func TestCreateEnvironmentRejectsUnwritableRWWorkspace(t *testing.T) {
	writeErr := errors.New("permission denied")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "profile" && args[1] == "show" {
			return rootProfileResult(), nil
		}
		if call == 6 {
			return host.Result{ExitCode: 1, Stderr: "permission denied"}, writeErr
		}
		return host.Result{}, nil
	}}
	_, err := New(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace"})
	if !errors.Is(err, core.ErrUnsupported) {
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
