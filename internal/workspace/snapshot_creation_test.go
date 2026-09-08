package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type snapshotCreateRuntime struct {
	*receiptRuntime
	mode string
}

func (r *snapshotCreateRuntime) CreateEnvironmentFromSnapshot(ctx context.Context, spec core.EnvironmentRuntimeSpec, saved core.Snapshot, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	lease, err := r.store.GetWorkspaceLease(ctx, spec.Name)
	if err != nil || lease.SnapshotSource != saved.ID || lease.InstanceID != spec.InstanceID || lease.InstanceID == saved.Source.InstanceID {
		r.t.Fatal("source/generation not reserved", lease, err)
	}
	if err := r.store.BeginSnapshotDelete(ctx, saved.ID); !errors.Is(err, core.ErrStorageBusy) {
		r.t.Fatal("source deletion raced creation", err)
	}
	if r.mode == "before" {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	if r.mode == "uncertain" {
		return core.EnvironmentRuntime{}, core.ErrRecoveryRequired
	}
	return r.receiptRuntime.CreateEnvironmentWithReceipt(ctx, spec, record)
}
func TestSnapshotCanonicalCreationOwnsSourceAndFailureCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "before", "uncertain", "configuration", "cleanup"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			capture, st, _ := captureFixture(t)
			saved, err := capture.CaptureSnapshot(ctx, "resume")
			if err != nil {
				t.Fatal(err)
			}
			if err = st.FinalizeEnvironmentDelete(ctx, "resume"); err != nil {
				t.Fatal(err)
			}
			base := &receiptRuntime{fakeEnvironmentRuntime: &fakeEnvironmentRuntime{}, t: t, store: st.EnvironmentJSONStore, fail: mode == "configuration" || mode == "cleanup"}
			if mode == "cleanup" {
				base.deleteErr = core.ErrRuntimeUnavailable
			}
			rt := &snapshotCreateRuntime{receiptRuntime: base, mode: mode}
			env, err := New(rt, st).CreateFromSnapshot(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()}, saved.ID)
			lease, leaseErr := st.GetWorkspaceLease(ctx, "demo")
			held := mode == "uncertain" || mode == "cleanup"
			if mode == "ok" {
				if err != nil || env.RuntimeRef != "haco-demo" || leaseErr != nil || lease.SnapshotSource != "" {
					t.Fatal("publication", env, lease, err, leaseErr)
				}
			} else if err == nil {
				t.Fatal("failure accepted")
			}
			if held {
				if !errors.Is(err, core.ErrRecoveryRequired) || leaseErr != nil || lease.SnapshotSource != saved.ID || lease.State != core.WorkspaceLeaseCleanupRequired {
					t.Fatal("lost ownership", lease, err, leaseErr)
				}
			} else if mode != "ok" && !errors.Is(leaseErr, core.ErrNotFound) {
				t.Fatal("cleaned lease retained", leaseErr)
			}
			deleteErr := st.BeginSnapshotDelete(ctx, saved.ID)
			if held {
				if !errors.Is(deleteErr, core.ErrStorageBusy) {
					t.Fatal("source released", deleteErr)
				}
			} else if deleteErr != nil {
				t.Fatal("source leaked", deleteErr)
			}
			if (mode == "configuration" || mode == "cleanup") && len(base.deleteRefs) != 1 {
				t.Fatal("cleanup not owned once", base.deleteRefs)
			}
		})
	}
}
