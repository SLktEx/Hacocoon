package run

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type maintenanceEnvironments struct {
	*fakeEnvironments
	create func(context.Context, core.EnvironmentSpec) (core.Environment, error)
	remove func(context.Context, string, core.Workspace) error
}

func (f maintenanceEnvironments) Create(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	return f.create(ctx, spec)
}
func (f maintenanceEnvironments) DeleteTemporary(ctx context.Context, name string, work core.Workspace) error {
	return f.remove(ctx, name, work)
}

func TestMaintainResourceSharesRunOwnershipAndCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "operation-failure", "cancel", "stale", "cleanup-failure", "create-failure"} {
		t.Run(mode, func(t *testing.T) {
			resource := core.PersistentResourceRef{ID: "oci:retained", Owner: strings.Repeat("a", 32)}
			store := newFakeRunStore()
			acted, removed, cleaned := false, false, false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runtime := maintenanceEnvironments{fakeEnvironments: &fakeEnvironments{}}
			runtime.create = func(ctx context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
				marker := store.runs[spec.Name]
				if marker.State != core.EphemeralRunCreating || marker.TemporaryWorkspace == nil || spec.TemporaryWorkspace == nil || *marker.TemporaryWorkspace != *spec.TemporaryWorkspace || spec.PersistentResource != resource.ID || spec.ExpectedResource != resource || spec.SkipDefaultResource || spec.WorkspacePath != "" {
					t.Fatal("maintenance reservation not pinned", spec)
				}
				if mode == "create-failure" {
					return core.Environment{}, core.ErrStorageBusy
				}
				env := core.Environment{Name: spec.Name, Workspace: *spec.TemporaryWorkspace, PersistentResource: resource}
				if mode == "stale" {
					env.PersistentResource.Owner = strings.Repeat("b", 32)
				}
				return env, nil
			}
			runtime.remove = func(ctx context.Context, name string, work core.Workspace) error {
				marker := store.runs[name]
				if marker.TemporaryWorkspace == nil || *marker.TemporaryWorkspace != work || ctx.Err() != nil {
					t.Fatal("cleanup lost live ownership or cancellation isolation")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("cleanup unbounded")
				}
				removed = true
				if mode == "cleanup-failure" {
					return core.ErrRecoveryRequired
				}
				return nil
			}
			service := NewWithRecovery(runtime, store, t.TempDir())
			service.newName = func() (string, error) { return "run-maintenance", nil }
			service.acquireOwnership = func(string, string, bool) (runOwnershipLock, bool, error) { return &fakeOwnershipLock{}, true, nil }
			service.ConfigureTemporaryWorkspace(func(ctx context.Context, work core.Workspace) error {
				if mode != "create-failure" && !removed {
					t.Fatal("data cleanup preceded runtime absence")
				}
				if !core.ValidTemporaryWorkspace(work) || ctx.Err() != nil {
					t.Fatal("wrong cleanup target")
				}
				cleaned = true
				return nil
			})
			result, err := service.MaintainResource(ctx, resource, func(ctx context.Context, env core.Environment) error {
				acted = true
				if store.runs[env.Name].State != core.EphemeralRunActive || removed || cleaned || env.PersistentResource != resource {
					t.Fatal("operation not owned")
				}
				if mode == "cancel" {
					cancel()
					return ctx.Err()
				}
				if mode == "operation-failure" {
					return core.ErrRuntimeUnavailable
				}
				return nil
			})
			if (mode == "ok") != (err == nil) {
				t.Fatal(err)
			}
			if (mode == "stale" || mode == "create-failure") == acted {
				t.Fatal("unsafe action dispatch", acted)
			}
			if mode == "cleanup-failure" {
				if result.CleanedUp || cleaned || store.runs["run-maintenance"].State != core.EphemeralRunCleanupRequired {
					t.Fatal("uncertain cleanup lost evidence")
				}
			} else if len(store.runs) != 0 || !cleaned {
				t.Fatal("completed cleanup not finalized")
			}
			if mode == "stale" && !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal(err)
			}
			if len(runtime.calls) != 0 {
				t.Fatal("maintenance used ordinary command/delete path", runtime.calls)
			}
		})
	}
}
func TestMaintainResourceRequiresDurableOwnershipAndValidSelection(t *testing.T) {
	resource := core.PersistentResourceRef{ID: "oci:retained", Owner: strings.Repeat("a", 32)}
	operation := func(context.Context, core.Environment) error { t.Fatal("unsupported maintenance executed"); return nil }
	for _, service := range []*Service{nil, New(&fakeEnvironments{})} {
		if _, err := service.MaintainResource(context.Background(), resource, operation); !errors.Is(err, core.ErrUnsupported) {
			t.Fatal(err)
		}
	}
	service := New(&fakeEnvironments{})
	if _, err := service.MaintainResource(context.Background(), core.PersistentResourceRef{}, operation); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := service.MaintainResource(context.Background(), resource, nil); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}
