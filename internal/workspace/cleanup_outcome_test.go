package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

func TestDeleteRetainsAggregateWhenAbsentRuntimeStillNeedsCleanup(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "pending"}[pending], func(t *testing.T) {
			ctx := context.Background()
			st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			rt := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
			svc := New(rt, st)
			env, err := svc.Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			lease, err := st.GetWorkspaceLease(ctx, "demo")
			if err != nil {
				t.Fatal(err)
			}
			if pending {
				// Retain the same canonical reservation without published metadata.
				if err := st.FinalizeEnvironmentDelete(ctx, "demo"); err != nil {
					t.Fatal(err)
				}
				lease.State, lease.RuntimeRef = core.WorkspaceLeaseAcquiring, ""
				if err := st.BeginEnvironmentCreate(ctx, lease); err != nil {
					t.Fatal(err)
				}
				lease.RuntimeRef = env.RuntimeRef
				if err := st.RecordEnvironmentRuntime(ctx, lease); err != nil {
					t.Fatal(err)
				}
			}
			guardFailure := errors.New("source guard cleanup failed")
			rt.deleteErr = errors.Join(core.ErrNotFound, guardFailure, core.ErrRecoveryRequired)
			err = svc.Delete(ctx, "demo")
			if !errors.Is(err, guardFailure) || !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatalf("ambiguous cleanup became success: %v", err)
			}
			held, err := st.GetWorkspaceLease(ctx, "demo")
			if err != nil || held.RuntimeRef != env.RuntimeRef || held.InstanceID != lease.InstanceID || held.State != core.WorkspaceLeaseCleanupRequired {
				t.Fatalf("ownership lost: %#v, %v", held, err)
			}
			if !pending {
				if current, err := st.GetEnvironment(ctx, "demo"); err != nil || current != env {
					t.Fatalf("ready data lost: %#v %v", current, err)
				}
			}
			if _, err := svc.Create(ctx, core.EnvironmentSpec{Name: "other", WorkspacePath: env.Workspace.Path}); !errors.Is(err, core.ErrWorkspaceBusy) {
				t.Fatalf("reserved Workspace reusable: %v", err)
			}
			rt.deleteErr = core.ErrNotFound
			if err := svc.Delete(ctx, "demo"); err != nil {
				t.Fatal(err)
			}
			if _, err := st.GetWorkspaceLease(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("retry failed: %v", err)
			}
		})
	}
}

func TestCreateFailureRetainsOwnershipWhenAbsentRuntimeStillNeedsCleanup(t *testing.T) {
	ctx := context.Background()
	st := newFakeEnvironmentStore()
	st.putErr = errors.New("publication failed")
	guardFailure := errors.New("source guard cleanup failed")
	rt := &fakeEnvironmentRuntime{
		createResult: core.EnvironmentRuntime{Ref: "haco-demo"},
		deleteErr:    errors.Join(core.ErrNotFound, guardFailure, core.ErrRecoveryRequired),
	}
	_, err := New(rt, st).Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()})
	if !errors.Is(err, st.putErr) || !errors.Is(err, guardFailure) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("cleanup failure lost: %v", err)
	}
	lease, err := st.GetWorkspaceLease(ctx, "demo")
	if err != nil || lease.State != core.WorkspaceLeaseCleanupRequired || lease.RuntimeRef != "haco-demo" {
		t.Fatalf("reservation lost: %#v %v", lease, err)
	}
}
