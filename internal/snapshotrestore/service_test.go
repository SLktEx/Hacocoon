package snapshotrestore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type fixture struct {
	fail     string
	cancel   context.CancelFunc
	steps    []string
	saved    core.Snapshot
	object   gitrepo.Object
	resource core.PersistentResource
	spec     core.EnvironmentSpec
}

func (f *fixture) GetSnapshot(context.Context, string) (core.Snapshot, error) { return f.saved, nil }
func (f *fixture) GetEnvironment(context.Context, string) (core.Environment, error) {
	if f.fail == "existing" {
		return core.Environment{Name: "other"}, nil
	}
	return core.Environment{}, core.ErrNotFound
}
func (f *fixture) GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error) {
	if f.fail == "lease" {
		return core.WorkspaceLease{}, nil
	}
	return core.WorkspaceLease{}, core.ErrNotFound
}
func (f *fixture) RestoreWorkspace(_ context.Context, id string, saved core.Snapshot) (gitrepo.Object, error) {
	f.steps = append(f.steps, "work")
	f.object = gitrepo.Object{ID: id, Owner: strings.Repeat("a", 32), State: "ready", RestoredFrom: saved.ID}
	if f.fail == "work" {
		return gitrepo.Object{}, core.ErrRuntimeUnavailable
	}
	return f.object, nil
}
func (f *fixture) DeleteRestoredCopy(_ context.Context, o gitrepo.Object) error {
	f.steps = append(f.steps, "delete-work")
	if o.ID != f.object.ID || o.Owner != f.object.Owner {
		return core.ErrCapabilityStale
	}
	return nil
}

// Separate Store adapter keeps the two concrete ownership contracts distinct.
type stores struct{ f *fixture }

func (s stores) RestoreSnapshot(_ context.Context, id string, _ core.Snapshot, w core.WorkspaceID) (core.PersistentResource, error) {
	f := s.f
	f.steps = append(f.steps, "oci")
	f.resource = core.PersistentResource{ID: id, Owner: strings.Repeat("b", 32), WorkspaceID: w, State: "ready"}
	if f.fail == "oci" {
		return core.PersistentResource{}, core.ErrRuntimeUnavailable
	}
	return f.resource, nil
}
func (s stores) DeleteRestoredCopy(_ context.Context, r core.PersistentResource) error {
	s.f.steps = append(s.f.steps, "delete-oci")
	if r.Owner != s.f.resource.Owner {
		return core.ErrCapabilityStale
	}
	return nil
}
func (f *fixture) CreateFromSnapshot(_ context.Context, spec core.EnvironmentSpec, id string) (core.Environment, error) {
	f.steps = append(f.steps, "create")
	f.spec = spec
	if f.cancel != nil {
		f.cancel()
		return core.Environment{}, context.Canceled
	}
	if f.fail == "create" || f.fail == "cleanup" {
		return core.Environment{}, core.ErrRuntimeUnavailable
	}
	return core.Environment{Name: spec.Name}, nil
}
func (f *fixture) StartForWorkspace(_ context.Context, _ string, w core.WorkspaceID) error {
	f.steps = append(f.steps, "start")
	if w != core.WorkspaceID("workspace:managed:"+f.object.Owner) {
		return core.ErrCapabilityStale
	}
	if f.fail == "start" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (f *fixture) CleanupRestoredData(ctx context.Context, w core.Workspace, remove func(context.Context) error) error {
	f.steps = append(f.steps, "cleanup")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if f.fail == "cleanup" {
		return core.ErrStorageBusy
	}
	return remove(ctx)
}
func TestRestorePublicWorkflowAndFailureOwnership(t *testing.T) {
	for _, mode := range []string{"ok", "no-oci", "existing", "lease", "work", "oci", "create", "cancel", "cleanup", "start"} {
		t.Run(mode, func(t *testing.T) {
			f := &fixture{fail: mode, saved: core.Snapshot{ID: "snap-" + strings.Repeat("c", 32), State: "ready", Source: core.SnapshotSource{Environment: core.Environment{Name: "dev", PersistentResource: core.PersistentResourceRef{ID: "oci:saved"}}}}}
			if mode == "no-oci" {
				f.saved.Source.Environment.PersistentResource = core.PersistentResourceRef{}
			}
			s := Service{Catalog: f, Environments: f, Workspaces: f, Stores: stores{f}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				f.cancel = cancel
			}
			result, err := s.RestoreSnapshot(ctx, f.saved.ID, "")
			calls := strings.Join(f.steps, ",")
			switch mode {
			case "ok", "no-oci":
				if err != nil || result.State != "running" || result.Environment != "dev-restored" || f.spec.Base != "" || f.spec.WorkspacePath != "managed:"+result.Workspace {
					t.Fatal(result, err, calls)
				}
				if mode == "no-oci" && (!f.spec.SkipDefaultResource || strings.Contains(calls, "oci")) {
					t.Fatal("implicit OCI copy", calls)
				}
			case "existing", "lease":
				if err == nil || len(f.steps) != 0 {
					t.Fatal("existing target changed", result, err, calls)
				}
			case "work":
				if err == nil || calls != "work" {
					t.Fatal(result, err, calls)
				}
			case "oci", "create", "cancel":
				if err == nil || result.Workspace != "" || result.OCI != "" || !strings.Contains(calls, "delete-work") {
					t.Fatal(result, err, calls)
				}
			case "cleanup":
				if !errors.Is(err, core.ErrRecoveryRequired) || result.Workspace == "" || result.OCI == "" || strings.Contains(calls, "delete-") {
					t.Fatal("leased data removed", result, err, calls)
				}
			case "start":
				if err == nil || result.State != "start-failed" || strings.Contains(calls, "cleanup") {
					t.Fatal("published Env rolled back", result, err, calls)
				}
			}
		})
	}
}
