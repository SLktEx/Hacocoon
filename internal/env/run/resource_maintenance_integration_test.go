package run

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
)

// Keep the real lifecycle validation and catalog transactions here. Only native
// resource execution is replaced; this is not real Incus acceptance.
func TestMaintainResourceUsesCanonicalLifecycleWithBorrowedStore(t *testing.T) {
	for _, operationFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "operation-failure"}[operationFails], func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HACO_ROOT", root)
			ctx := context.Background()
			catalog := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
			resource := core.PersistentResource{ID: "oci:retained", Owner: strings.Repeat("a", 32), Kind: "oci-store", NativeRef: "test/retained", WorkspaceID: "original-workspace", State: "creating", CreatedAt: time.Now().UTC()}
			if err := catalog.BeginPersistentResourceCreate(ctx, resource); err != nil {
				t.Fatal(err)
			}
			if err := catalog.CommitPersistentResourceCreate(ctx, resource); err != nil {
				t.Fatal(err)
			}
			resource.State = "ready"
			native := &maintenanceNativeBoundary{}
			environments := workspace.New(native, catalog)
			environments.ConfigureDefaultResource(func(context.Context, core.Workspace) (core.PersistentResource, error) {
				t.Fatal("borrowed Store triggered default Store provisioning")
				return core.PersistentResource{}, core.ErrInvalidArgument
			})
			service := NewWithRecovery(environments, catalog, filepath.Join(root, "run-locks"))
			cleaned := false
			service.ConfigureTemporaryWorkspace(func(ctx context.Context, work core.Workspace) error {
				if native.live || !core.ValidTemporaryWorkspace(work) {
					t.Fatal("cleanup before exact runtime absence")
				}
				cleaned = true
				return nil
			})
			acted := false
			result, err := service.MaintainResource(ctx, resource.Ref(), func(ctx context.Context, env core.Environment) error {
				acted = true
				lease, e := catalog.GetWorkspaceLease(ctx, env.Name)
				if e != nil || lease.State != core.WorkspaceLeaseActive || lease.PersistentResource != resource.Ref() || !native.live || !native.spec.ResourceMaintenance {
					t.Fatal("missing real lifecycle reservation", e)
				}
				if _, e = catalog.BeginPersistentResourceDeleteReviewed(ctx, resource.Ref()); !errors.Is(e, core.ErrStorageBusy) {
					t.Fatal("borrowed Store deletion was not refused", e)
				}
				if operationFails {
					return core.ErrRuntimeUnavailable
				}
				return nil
			})
			if !acted || !cleaned || native.live || !result.CleanedUp || (operationFails != errors.Is(err, core.ErrRuntimeUnavailable)) || (!operationFails && err != nil) {
				t.Fatal("maintenance lifecycle failed", result, err, acted, cleaned)
			}
			retained, e := catalog.GetPersistentResource(ctx, resource.ID)
			if e != nil || retained != resource {
				t.Fatal("borrowed Store or original association changed", e)
			}
			envs, e := catalog.ListEnvironments(ctx)
			if e != nil || len(envs) != 0 {
				t.Fatal("environment metadata retained", e)
			}
			leases, e := catalog.ListWorkspaceLeases(ctx)
			if e != nil || len(leases) != 0 {
				t.Fatal("lease retained", e)
			}
			runs, e := catalog.ListEphemeralRuns(ctx)
			if e != nil || len(runs) != 0 {
				t.Fatal("run ownership retained", e)
			}
		})
	}
}

type maintenanceNativeBoundary struct {
	live bool
	spec core.EnvironmentRuntimeSpec
}

func (n *maintenanceNativeBoundary) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{}, core.ErrUnsupported
}
func (n *maintenanceNativeBoundary) CreateEnvironmentWithReceipt(ctx context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if !spec.ResourceMaintenance || !spec.TemporaryWorkspace || !core.ValidPersistentResourceRef(spec.PersistentResource.Ref()) {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	n.live, n.spec = true, spec
	value := core.EnvironmentRuntime{Ref: "test:" + spec.Name, Resources: spec.Resources}
	return value, record(value)
}
func (n *maintenanceNativeBoundary) DeleteEnvironment(_ context.Context, ref string) error {
	if !n.live || ref != "test:"+n.spec.Name {
		return core.ErrCapabilityStale
	}
	n.live = false
	return nil
}
func (*maintenanceNativeBoundary) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, core.ErrUnsupported
}
func (*maintenanceNativeBoundary) ShellEnvironment(context.Context, string) error {
	return core.ErrUnsupported
}
