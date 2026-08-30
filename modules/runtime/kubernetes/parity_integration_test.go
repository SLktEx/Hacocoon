package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/internal/host"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
)

func TestWorkspaceLeaseParityThroughKubernetesProvider(t *testing.T) {
	ctx := context.Background()
	workspaceDir := t.TempDir()
	runner := newParityKubeRunner(t)
	environments, store := newParityEnvironmentService(t, runner)

	first, err := environments.Create(ctx, core.EnvironmentSpec{
		Name:          "one",
		WorkspacePath: workspaceDir,
		AccessMode:    core.WorkspaceReadWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.RuntimeRef, "haco-runtime-v1:"+ProviderID+":") {
		t.Fatalf("runtime ref = %q", first.RuntimeRef)
	}

	callsBeforeConflict := len(runner.calls)
	_, err = environments.Create(ctx, core.EnvironmentSpec{
		Name:          "two",
		WorkspacePath: workspaceDir,
		AccessMode:    core.WorkspaceReadOnly,
	})
	if !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("RW/RO conflict error = %v, want ErrWorkspaceBusy", err)
	}
	if len(runner.calls) != callsBeforeConflict {
		t.Fatalf("workspace conflict touched Kubernetes: before=%d after=%d", callsBeforeConflict, len(runner.calls))
	}

	lease, err := store.GetWorkspaceLease(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	if lease.State != core.WorkspaceLeaseActive || lease.AccessMode != core.WorkspaceReadWrite || lease.RuntimeRef != first.RuntimeRef {
		t.Fatalf("lease = %#v", lease)
	}

	if err := environments.Delete(ctx, "one"); err != nil {
		t.Fatal(err)
	}
	second, err := environments.Create(ctx, core.EnvironmentSpec{
		Name:          "two",
		WorkspacePath: workspaceDir,
		AccessMode:    core.WorkspaceReadOnly,
	})
	if err != nil {
		t.Fatalf("create after lease release: %v", err)
	}
	if second.AccessMode != core.WorkspaceReadOnly {
		t.Fatalf("access mode = %q", second.AccessMode)
	}
}

func TestReadOnlyWorkspaceParityAllowsMultipleKubernetesEnvironments(t *testing.T) {
	ctx := context.Background()
	workspaceDir := t.TempDir()
	runner := newParityKubeRunner(t)
	environments, _ := newParityEnvironmentService(t, runner)

	for _, name := range []string{"reader-one", "reader-two"} {
		if _, err := environments.Create(ctx, core.EnvironmentSpec{
			Name:          name,
			WorkspacePath: workspaceDir,
			AccessMode:    core.WorkspaceReadOnly,
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
}

func TestEphemeralRunParityThroughKubernetesProvider(t *testing.T) {
	ctx := context.Background()
	workspaceDir := t.TempDir()
	runner := newParityKubeRunner(t)
	environments, store := newParityEnvironmentService(t, runner)
	runs := runapp.NewWithRecovery(environments, store, filepath.Join(t.TempDir(), "run-locks"))

	result, err := runs.Run(ctx, runapp.Spec{
		WorkspacePath: workspaceDir,
		AccessMode:    core.WorkspaceReadWrite,
		Argv:          []string{"printf", "parity-ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Environment == "" || !result.CleanedUp {
		t.Fatalf("run result = %#v", result)
	}
	if result.Execution.Stdout != "parity-ok" {
		t.Fatalf("stdout = %q", result.Execution.Stdout)
	}
	if _, err := store.GetEnvironment(ctx, result.Environment); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("ephemeral Environment persisted after cleanup: %v", err)
	}
	ephemeral, err := store.ListEphemeralRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ephemeral) != 0 {
		t.Fatalf("ephemeral run markers remain: %#v", ephemeral)
	}
}

func TestFiniteResourceBudgetSurvivesProviderRouting(t *testing.T) {
	ctx := context.Background()
	workspaceDir := t.TempDir()
	runner := newParityKubeRunner(t)
	environments, store := newParityEnvironmentService(t, runner)
	requested := core.ResourceBudget{
		CPU:         core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 2},
		MemoryBytes: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 512 << 20},
		RootBytes:   core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 8 << 30},
	}

	created, err := environments.Create(ctx, core.EnvironmentSpec{
		Name:          "limited",
		WorkspacePath: workspaceDir,
		AccessMode:    core.WorkspaceReadWrite,
		Resources:     requested,
	})
	if err != nil {
		t.Fatal(err)
	}
	effective, err := core.ResolveResourceBudget(requested)
	if err != nil {
		t.Fatal(err)
	}
	if created.Resources != effective {
		t.Fatalf("created resources = %#v, want %#v", created.Resources, effective)
	}
	persisted, err := store.GetEnvironment(ctx, "limited")
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Resources != effective {
		t.Fatalf("persisted resources = %#v, want %#v", persisted.Resources, effective)
	}
}

func newParityEnvironmentService(t *testing.T, runner host.Runner) (*workspaceapp.Service, *state.EnvironmentJSONStore) {
	t.Helper()
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	if err != nil {
		t.Fatal(err)
	}
	router, err := environmentapp.NewRouter(ProviderID, environmentapp.Register(ProviderID, provider))
	if err != nil {
		t.Fatal(err)
	}
	baseRouter := environmentapp.NewBaseRouter(router)
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	return workspaceapp.New(baseRouter, store), store
}

func newParityKubeRunner(t *testing.T) *fakeRunner {
	t.Helper()
	namespaces := map[string]string{}
	runner := &fakeRunner{}
	runner.run = func(_ context.Context, name string, args []string) (host.Result, error) {
		if name != defaultKubectl {
			return host.Result{}, errors.New("unexpected executable")
		}
		switch {
		case len(args) == 6 && args[0] == "get" && args[1] == "namespace" && args[3] == "--ignore-not-found":
			ref := args[2]
			environmentName, ok := namespaces[ref]
			if !ok {
				return host.Result{}, nil
			}
			state := namespaceState{}
			state.Metadata.Name = ref
			state.Metadata.Labels = managedLabels(environmentName)
			data, err := json.Marshal(state)
			return host.Result{Stdout: string(data)}, err

		case len(args) == 3 && args[0] == "create" && args[1] == "-f":
			data, err := os.ReadFile(args[2])
			if err != nil {
				return host.Result{}, err
			}
			var manifest map[string]any
			if err := json.Unmarshal(data, &manifest); err != nil {
				return host.Result{}, err
			}
			if manifest["kind"] == "Namespace" {
				metadata := manifest["metadata"].(map[string]any)
				ref := metadata["name"].(string)
				labels := metadata["labels"].(map[string]any)
				namespaces[ref] = labels[environmentLabel].(string)
			}
			return host.Result{}, nil

		case len(args) >= 5 && args[0] == "-n" && args[2] == "wait":
			return host.Result{}, nil

		case len(args) >= 6 && args[0] == "-n" && args[2] == "exec" && args[4] == "--":
			command := args[5:]
			if reflect.DeepEqual(command, []string{"cat", "/proc/1/comm"}) {
				return host.Result{Stdout: "systemd\n"}, nil
			}
			if reflect.DeepEqual(command, []string{"test", "-w", "/workspace"}) {
				return host.Result{}, nil
			}
			if reflect.DeepEqual(command, []string{"printf", "parity-ok"}) {
				return host.Result{Stdout: "parity-ok"}, nil
			}
			return host.Result{}, nil

		case len(args) >= 3 && args[0] == "delete" && args[1] == "namespace":
			delete(namespaces, args[2])
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected kubectl call: " + strings.Join(args, " "))
		}
	}
	return runner
}
