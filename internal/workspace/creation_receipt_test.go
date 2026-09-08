package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"path/filepath"
	"testing"
)

type receiptRuntime struct {
	*fakeEnvironmentRuntime
	t     *testing.T
	store *state.EnvironmentJSONStore
	fail  bool
}

func (r *receiptRuntime) CreateEnvironmentWithReceipt(ctx context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	created := core.EnvironmentRuntime{Ref: "haco-demo", Resources: spec.Resources}
	if err := record(created); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	// Model the first post-init configuration step: the real store must already
	// own the exact provider resource while keeping the Workspace unpublished.
	lease, err := r.store.GetWorkspaceLease(ctx, spec.Name)
	if err != nil || lease.RuntimeRef != created.Ref || lease.InstanceID != spec.InstanceID || lease.State != core.WorkspaceLeaseAcquiring {
		r.t.Fatal("configuration preceded durable ownership", lease, err)
	}
	if _, err := r.store.GetEnvironment(ctx, spec.Name); !errors.Is(err, core.ErrNotFound) {
		r.t.Fatal("premature Environment publication", err)
	}
	if r.fail {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	return created, nil
}
func TestCreationReceiptPrecedesConfigurationAndProtectsFailedCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "configuration-failed", "cleanup-failed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			r := &receiptRuntime{fakeEnvironmentRuntime: &fakeEnvironmentRuntime{}, t: t, store: store, fail: mode != "ok"}
			if mode == "cleanup-failed" {
				r.deleteErr = core.ErrRuntimeUnavailable
			}
			_, err := New(r, store).Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir()})
			if mode == "ok" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || len(r.deleteRefs) != 1 || r.deleteRefs[0] != "haco-demo" {
				t.Fatal("configuration failure not cleaned", err, r.deleteRefs)
			}
			lease, readErr := store.GetWorkspaceLease(ctx, "demo")
			if mode == "cleanup-failed" {
				if !errors.Is(err, core.ErrRecoveryRequired) || readErr != nil || lease.RuntimeRef != "haco-demo" || lease.State != core.WorkspaceLeaseCleanupRequired {
					t.Fatal("lost owned runtime", lease, err, readErr)
				}
			} else if !errors.Is(readErr, core.ErrNotFound) {
				t.Fatal("cleaned reservation retained", readErr)
			}
		})
	}
}
