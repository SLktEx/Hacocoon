package workspace

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type archiveCreateRuntime struct {
	*receiptRuntime
	mode string
}

func (r *archiveCreateRuntime) CreateEnvironmentFromArchive(ctx context.Context, spec core.EnvironmentRuntimeSpec, source io.ReadSeeker, privateRoot string, limit int64, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	lease, err := r.store.GetWorkspaceLease(ctx, spec.Name)
	if err != nil || lease.SnapshotSource != "" || lease.InstanceID != spec.InstanceID || !core.ValidEnvironmentInstanceID(spec.InstanceID) {
		r.t.Fatal("generation not reserved", lease, err)
	}
	data, err := io.ReadAll(source)
	if err != nil || string(data) != "rootfs" || limit != 1024 {
		r.t.Fatal("source changed", err)
	}
	if r.mode == "before" {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	if r.mode == "uncertain" {
		return core.EnvironmentRuntime{}, core.ErrRecoveryRequired
	}
	return r.receiptRuntime.CreateEnvironmentWithReceipt(ctx, spec, record)
}
func TestArchiveCanonicalCreationOwnsFailureCleanup(t *testing.T) {
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
			rt := &archiveCreateRuntime{receiptRuntime: base, mode: mode}
			env, err := New(rt, st).CreateFromArchive(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()}, bytes.NewReader([]byte("rootfs")), t.TempDir(), 1024)
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
				if !errors.Is(err, core.ErrRecoveryRequired) || leaseErr != nil || lease.SnapshotSource != "" || lease.State != core.WorkspaceLeaseCleanupRequired {
					t.Fatal("lost ownership", lease, err, leaseErr)
				}
			} else if mode != "ok" && !errors.Is(leaseErr, core.ErrNotFound) {
				t.Fatal("cleaned lease retained", leaseErr)
			}
			// Archive creation never reserves or mutates an unrelated saved snapshot.
			if err := st.BeginSnapshotDelete(ctx, saved.ID); err != nil {
				t.Fatal("archive import acquired snapshot", err)
			}
			if (mode == "configuration" || mode == "cleanup") && len(base.deleteRefs) != 1 {
				t.Fatal("cleanup not owned once", base.deleteRefs)
			}
		})
	}
}

func TestArchiveRecreationKeepsWorkspaceAndRenewsGeneration(t *testing.T) {
	ctx := context.Background()
	st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	rt := &archiveCreateRuntime{receiptRuntime: &receiptRuntime{fakeEnvironmentRuntime: &fakeEnvironmentRuntime{}, t: t, store: st}}
	service := New(rt, st)
	service.defaultResource = func(context.Context, core.Workspace) (core.PersistentResource, error) {
		t.Fatal("import copied current Host OCI")
		return core.PersistentResource{}, nil
	}
	work := t.TempDir()
	retained := filepath.Join(work, "untracked")
	if err := os.WriteFile(retained, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	var previous string
	for i := 0; i < 2; i++ {
		env, err := service.CreateFromArchive(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: work}, bytes.NewReader([]byte("rootfs")), t.TempDir(), 1024)
		if err != nil || env.Base != nil || env.PersistentResource.ID != "" {
			t.Fatal(env, err)
		}
		lease, err := st.GetWorkspaceLease(ctx, "demo")
		if err != nil || lease.InstanceID == previous {
			t.Fatal("reused authority", lease, err)
		}
		previous = lease.InstanceID
		if err := service.Delete(ctx, "demo"); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(retained)
		if err != nil || string(data) != "retained" {
			t.Fatal("workspace lost", err)
		}
	}
}
