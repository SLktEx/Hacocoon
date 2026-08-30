package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnsureTrustedHostCreatesMarkedInstanceOnManagedPoolAndStartsIt(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "storage" && args[1] == "show":
			return host.Result{}, nil
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: "[]\n"}, nil
		case len(args) >= 4 && args[0] == "config" && args[1] == "get" && args[2] == trustedHostName:
			return host.Result{Stdout: trustedHostRoleValue + "\n"}, nil
		default:
			return host.Result{}, nil
		}
	}}
	runtime := New(runner)
	if err := runtime.ConfigureStorageProvider(func(context.Context) (map[string]string, error) {
		return map[string]string{
			"incus_pool": "haco-local-default",
			"driver":     "btrfs",
			"source":     "/var/lib/hacocoon/mnt/local-default",
		}, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := runtime.EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []runnerCall{
		{name: "incus", args: []string{"project", "show", defaultProject}},
		{name: "incus", args: []string{"storage", "show", "haco-local-default", "--project", defaultProject}},
		{name: "incus", args: []string{"list", trustedHostName, "--project", defaultProject, "--format", "json"}},
		{name: "incus", args: []string{"init", defaultImage, trustedHostName, "--project", defaultProject, "--storage", "haco-local-default", "--config", trustedHostRoleKey + "=" + trustedHostRoleValue}},
		{name: "incus", args: []string{"config", "get", trustedHostName, trustedHostRoleKey, "--project", defaultProject}},
		{name: "incus", args: []string{"start", trustedHostName, "--project", defaultProject}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v\nwant = %#v", runner.calls, want)
	}
}

func TestEnsureTrustedHostReusesRunningOwnedInstance(t *testing.T) {
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue)
	runtime := New(runner)

	if err := runtime.EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && (call.args[0] == "init" || call.args[0] == "start") {
			t.Fatalf("unexpected mutation for running host: %#v", call)
		}
	}
}

func TestEnsureTrustedHostStartsStoppedOwnedInstance(t *testing.T) {
	runner := trustedHostRunner("STOPPED", trustedHostRoleValue)
	if err := New(runner).EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}
	last := runner.calls[len(runner.calls)-1]
	assertRunnerCall(t, last, "incus", "start", trustedHostName, "--project", defaultProject)
}

func TestEnsureTrustedHostRefusesUnownedNameCollision(t *testing.T) {
	runner := trustedHostRunner("RUNNING", "")
	err := New(runner).EnsureTrustedHost(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && (call.args[0] == "init" || call.args[0] == "start") {
			t.Fatalf("unowned instance was mutated: %#v", call)
		}
	}
}

func TestEnsureTrustedHostRejectsUnexpectedState(t *testing.T) {
	runner := trustedHostRunner("FROZEN", trustedHostRoleValue)
	err := New(runner).EnsureTrustedHost(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureTrustedHostRecoversConcurrentCreateOfOwnedInstance(t *testing.T) {
	initErr := errors.New("instance already exists")
	listCalls := 0
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "profile" && args[1] == "show":
			return rootProfileResult(), nil
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			listCalls++
			if listCalls == 1 {
				return host.Result{Stdout: "[]\n"}, nil
			}
			return host.Result{Stdout: `[{"name":"haco-host","status":"RUNNING"}]`}, nil
		case len(args) > 0 && args[0] == "init":
			return host.Result{ExitCode: 1}, initErr
		case len(args) >= 4 && args[0] == "config" && args[1] == "get":
			return host.Result{Stdout: trustedHostRoleValue + "\n"}, nil
		default:
			return host.Result{}, nil
		}
	}}

	if err := New(runner).EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func trustedHostRunner(state, role string) *fakeRunner {
	return &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "profile" && args[1] == "show":
			return rootProfileResult(), nil
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: `[{"name":"haco-host","status":"` + state + `"}]`}, nil
		case len(args) >= 4 && args[0] == "config" && args[1] == "get" && args[2] == trustedHostName:
			return host.Result{Stdout: role + "\n"}, nil
		default:
			return host.Result{}, nil
		}
	}}
}
