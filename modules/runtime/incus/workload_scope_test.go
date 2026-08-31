package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestScopedWorkloadsPinsIncusProjectAtBrokerCreation(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{}, nil
	}}
	runtime := New(runner)
	scoped, err := runtime.BindEnvironmentWorkloads("demo")
	if err != nil {
		t.Fatal(err)
	}
	if scoped.Project() != defaultProject {
		t.Fatalf("bound project = %q, want %q", scoped.Project(), defaultProject)
	}

	// Simulate a future/reconfigured Runtime trying to point the same listener
	// at another Incus Project. Existing guest authority must not follow it.
	runtime.project = "other-project"
	_, err = scoped.CreateWorkload(context.Background(), core.WorkloadSpec{
		Environment: "demo",
		Name:        "db",
		Image:       "oci-docker:library/postgres:18",
	})
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("CreateWorkload error = %v, want policy denied", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("cross-project request reached Incus: %#v", runner.calls)
	}
}

func TestScopedWorkloadsRejectsGuestEnvironmentSelection(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{}, nil
	}}
	runtime := New(runner)
	scoped, err := runtime.BindEnvironmentWorkloads("demo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = scoped.CreateWorkload(context.Background(), core.WorkloadSpec{
		Environment: "other",
		Name:        "db",
		Image:       "oci-docker:library/postgres:18",
	})
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("CreateWorkload error = %v, want policy denied", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("cross-Environment request reached Incus: %#v", runner.calls)
	}
}

func TestScopedWorkloadsVerifiesEnvironmentInsideBoundProject(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 6 && args[0] == "list" && args[1] == "haco-demo" {
			if args[2] != "--project" || args[3] != defaultProject {
				t.Fatalf("Environment lookup escaped project: %#v", args)
			}
			return host.Result{Stdout: "haco-demo\n"}, nil
		}
		if len(args) >= 6 && args[0] == "config" && args[1] == "get" && args[2] == "haco-demo" && args[3] == managedEnvironmentMarkerKey {
			if args[4] != "--project" || args[5] != defaultProject {
				t.Fatalf("Environment marker lookup escaped project: %#v", args)
			}
			return host.Result{Stdout: managedEnvironmentMarkerValue + "\n"}, nil
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	scoped, err := runtime.BindEnvironmentWorkloads("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := scoped.authorize(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("authorization Incus calls = %d, want 2: %#v", len(runner.calls), runner.calls)
	}
}
