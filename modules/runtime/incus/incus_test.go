package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestCreateUsesPreparedImageAndOpaqueStorageAttachment(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)

	created, err := runtime.Create(context.Background(), core.RuntimeSessionSpec{
		ID:   "abc123",
		Name: "demo",
		StorageAttachment: map[string]string{
			"incus_pool": "haco-pool",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Ref != "haco-abc123" {
		t.Fatalf("ref = %q", created.Ref)
	}

	launch := findCall(t, runner.calls, "launch")
	want := []string{"launch", preparedImageAlias, "haco-abc123", "--project", defaultProject, "-c", "security.nesting=true", "--storage", "haco-pool"}
	if strings.Join(launch.args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("launch args = %v, want %v", launch.args, want)
	}
}

func TestEnsureStoragePoolCreatesFromOpaqueAttachment(t *testing.T) {
	runner := &fakeRunner{}
	runner.run = func(_ string, args []string) (host.Result, error) {
		if hasPrefix(args, "storage", "show") {
			return host.Result{}, errors.New("not found")
		}
		return host.Result{}, nil
	}
	runtime := New(runner)

	pool, err := runtime.ensureStoragePool(context.Background(), map[string]string{
		"incus_pool": "haco-pool",
		"driver":     "btrfs",
		"source":     "/dev/loop42",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pool != "haco-pool" {
		t.Fatalf("pool = %q", pool)
	}
	var created runnerCall
	found := false
	for _, call := range runner.calls {
		if len(call.args) >= 2 && call.args[0] == "storage" && call.args[1] == "create" {
			created = call
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing storage create call: %v", runner.calls)
	}
	if !containsSequence(created.args, "create", "haco-pool", "btrfs", "source=/dev/loop42") {
		t.Fatalf("storage create args = %v", created.args)
	}
}

func TestEnsureStoragePoolRejectsIncompleteAttachment(t *testing.T) {
	runner := &fakeRunner{}
	runner.run = func(_ string, args []string) (host.Result, error) {
		if hasPrefix(args, "storage", "show") {
			return host.Result{}, errors.New("not found")
		}
		return host.Result{}, nil
	}
	runtime := New(runner)

	_, err := runtime.ensureStoragePool(context.Background(), map[string]string{"incus_pool": "haco-pool", "driver": "btrfs"})
	if err == nil || !strings.Contains(err.Error(), "missing driver/source") {
		t.Fatalf("expected incomplete attachment error, got %v", err)
	}
	if hasCall(runner.calls, "create") {
		t.Fatalf("must not attempt creation with incomplete attachment: %v", runner.calls)
	}
}

func TestExecNonInteractivePreservesArgv(t *testing.T) {
	runner := &fakeRunner{}
	runner.run = func(_ string, args []string) (host.Result, error) {
		return host.Result{Stdout: "ok\n", Stderr: "warn\n", ExitCode: 7}, errors.New("exit 7")
	}
	runtime := New(runner)

	result, err := runtime.Exec(context.Background(), "haco-abc", core.ExecRequest{Argv: []string{"sh", "-c", "printf '%s' 'a b'"}})
	if err == nil {
		t.Fatal("expected runner error")
	}
	if result.ExitCode != 7 || result.Stdout != "ok\n" || result.Stderr != "warn\n" {
		t.Fatalf("result = %#v", result)
	}
	assertCall(t, runner.calls[0], "incus", "exec", "haco-abc", "--project", defaultProject, "--", "sh", "-c", "printf '%s' 'a b'")
}

func TestExecRejectsEmptyArgv(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	_, err := runtime.Exec(context.Background(), "haco-abc", core.ExecRequest{})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner must not be called: %v", runner.calls)
	}
}

func TestInspectMapsIncusStates(t *testing.T) {
	cases := []struct {
		stdout string
		want   core.ObservedState
	}{
		{"RUNNING\n", core.ObservedRunning},
		{"stopped\n", core.ObservedStopped},
		{"FROZEN\n", core.ObservedUnknown},
	}
	for _, tc := range cases {
		t.Run(strings.TrimSpace(tc.stdout), func(t *testing.T) {
			runner := &fakeRunner{run: func(_ string, _ []string) (host.Result, error) {
				return host.Result{Stdout: tc.stdout}, nil
			}}
			runtime := New(runner)
			state, err := runtime.Inspect(context.Background(), "haco-abc")
			if err != nil {
				t.Fatal(err)
			}
			if state.Observed != tc.want {
				t.Fatalf("observed = %q, want %q", state.Observed, tc.want)
			}
		})
	}
}

func TestLifecycleCommandsAreProjectScoped(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	ctx := context.Background()
	if err := runtime.Start(ctx, "haco-abc"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Stop(ctx, "haco-abc"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Delete(ctx, "haco-abc"); err != nil {
		t.Fatal(err)
	}

	assertCall(t, runner.calls[0], "incus", "start", "haco-abc", "--project", defaultProject)
	assertCall(t, runner.calls[1], "incus", "stop", "haco-abc", "--project", defaultProject)
	assertCall(t, runner.calls[2], "incus", "delete", "haco-abc", "--project", defaultProject, "--force")
}
