package incus

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestResourceMaintenancePreparationRefusesHostAndFailedObservation(t *testing.T) {
	for _, mode := range []string{"ok", "host", "option", "failure", "truncated", "error"} {
		t.Run(mode, func(t *testing.T) {
			called := false
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				called = true
				if name != "incus" || args[0] != "exec" || args[1] != "haco-maintenance" || args[len(args)-1] != resourceMaintenancePreparation {
					t.Fatal("unexpected authority", name, args)
				}
				switch mode {
				case "failure":
					return host.Result{ExitCode: 43}, nil
				case "truncated":
					return host.Result{StderrTruncated: true}, nil
				case "error":
					return host.Result{}, errors.New("unavailable")
				}
				return host.Result{}, nil
			}}
			p, err := NewSandboxProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			ref := "haco-maintenance"
			if mode == "host" {
				ref = trustedHostName
			}
			if mode == "option" {
				ref = "--project"
			}
			err = p.prepareResourceMaintenance(context.Background(), ref)
			if mode == "ok" {
				if err != nil || !called {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("unsafe preparation accepted")
			}
			if (mode == "host" || mode == "option") && (called || !errors.Is(err, core.ErrInvalidArgument)) {
				t.Fatal("unsafe target dispatched", err)
			}
		})
	}
}

func TestResourceMaintenancePreparationShellSyntax(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable; syntax only")
	}
	cmd := exec.Command(bash, "-n")
	cmd.Stdin = strings.NewReader(resourceMaintenancePreparation)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("syntax: %v %s", err, out)
	}
}

func TestResourceMaintenanceCreationRefusesBeforeNativeAccess(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("unfinished maintenance dispatched a native operation")
		return host.Result{}, core.ErrUnsupported
	}}
	runtime := New(runner)
	sandbox, err := NewSandboxProvider(runtime)
	if err != nil {
		t.Fatal(err)
	}
	spec := core.EnvironmentRuntimeSpec{ResourceMaintenance: true}
	for name, create := range map[string]func(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error){
		"runtime": runtime.CreateEnvironment, "base": sandbox.BaseProvider.CreateEnvironment, "sandbox": sandbox.CreateEnvironment,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := create(context.Background(), spec); !errors.Is(err, core.ErrUnsupported) {
				t.Fatal(err)
			}
		})
	}
}
