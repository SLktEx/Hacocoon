package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"path/filepath"
	"testing"
)

type creationLifetimeRuntime struct {
	fakeEnvironmentRuntime
	state core.EnvironmentState
	stops int
}

func (r *creationLifetimeRuntime) InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
	return core.EnvironmentRuntimeStatus{State: r.state}, nil
}
func (r *creationLifetimeRuntime) StopEnvironment(context.Context, string) error {
	r.stops++
	r.state = core.EnvironmentStopped
	return nil
}

func TestCreatedWorkspaceLifetimeAndExclusiveVolume(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "explicit-volume", true: "automatic-workspace"}[owned], func(t *testing.T) {
			ctx := context.Background()
			work := core.Workspace{ID: "workspace:managed:owned", Path: "managed:volume-data"}
			provider := &managedDeleteProvider{work: work}
			runtime := &creationLifetimeRuntime{fakeEnvironmentRuntime: fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-dev"}}, state: core.EnvironmentRunning}
			catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			service := NewWithProvider(runtime, catalog, provider)
			spec := core.EnvironmentSpec{Name: "dev", WorkspacePath: work.Path, ExpectedWorkspace: work.ID, SkipDefaultResource: true, DeferStart: true, OwnedWorkspace: owned}
			if !owned {
				spec.Volume = "data"
			}
			created, err := service.Create(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			if created.OwnedWorkspace != owned || created.Volume != spec.Volume || !runtime.createSpec.DeferStart {
				t.Fatal("lost immutable creation configuration", created)
			}
			other := spec
			other.Name = "other"
			if _, err := service.Create(ctx, other); err == nil {
				t.Fatal("same Workspace bound twice")
			}
			if err := service.DeleteUser(ctx, "dev", false); !errors.Is(err, core.ErrStorageBusy) || runtime.stops != 0 || len(runtime.deleteRefs) != 0 {
				t.Fatal("running Environment changed without force", err)
			}
			if err := service.DeleteUser(ctx, "dev", true); err != nil {
				t.Fatal(err)
			}
			if runtime.stops != 1 || len(runtime.deleteRefs) != 1 {
				t.Fatal("force did not stop and delete once")
			}
			expectedDeletes := 0
			if owned {
				expectedDeletes = 1
			}
			if provider.calls != expectedDeletes {
				t.Fatalf("Workspace deletes=%d want=%d", provider.calls, expectedDeletes)
			}
			if _, err := catalog.GetEnvironment(ctx, "dev"); !errors.Is(err, core.ErrNotFound) {
				t.Fatal("Environment retained", err)
			}
			if _, err := catalog.GetWorkspaceLease(ctx, "dev"); !errors.Is(err, core.ErrNotFound) {
				t.Fatal("Volume relation retained", err)
			}
		})
	}
}

type retryOwnedWorkspaceProvider struct {
	managedDeleteProvider
	fail bool
}

func (p *retryOwnedWorkspaceProvider) DeleteWorkspace(ctx context.Context, work core.Workspace) error {
	p.calls++
	if p.fail {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func TestEnvironmentDeleteRetriesOwnedDataAfterRuntimeRemoval(t *testing.T) {
	ctx := context.Background()
	work := core.Workspace{ID: "workspace:managed:owned", Path: "managed:work"}
	provider := &retryOwnedWorkspaceProvider{managedDeleteProvider: managedDeleteProvider{work: work}, fail: true}
	runtime := &creationLifetimeRuntime{fakeEnvironmentRuntime: fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "owned"}}, state: core.EnvironmentStopped}
	path := filepath.Join(t.TempDir(), "state.json")
	catalog := state.NewEnvironmentJSONStore(path)
	service := NewWithProvider(runtime, catalog, provider)
	spec := core.EnvironmentSpec{Name: "dev", WorkspacePath: work.Path, ExpectedWorkspace: work.ID, SkipDefaultResource: true, OwnedWorkspace: true, DeferStart: true}
	if _, err := service.Create(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteUser(ctx, "dev", false); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	reopened := state.NewEnvironmentJSONStore(path)
	if got, err := reopened.OwnedWorkspaceCleanup(ctx, "dev"); err != nil || got != work {
		t.Fatal("lost data ownership", got, err)
	}
	if _, err := service.Create(ctx, spec); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("reused unfinished deletion", err)
	}
	provider.fail = false
	service = NewWithProvider(runtime, reopened, provider)
	if err := service.DeleteUser(ctx, "dev", false); err != nil {
		t.Fatal(err)
	}
	if len(runtime.deleteRefs) != 1 || provider.calls != 2 {
		t.Fatal("repeated provider deletion or skipped retry")
	}
	if _, err := reopened.OwnedWorkspaceCleanup(ctx, "dev"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}
