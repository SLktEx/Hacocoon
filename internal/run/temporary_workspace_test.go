package run

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type temporaryEnvironments struct {
	*fakeEnvironments
	temporaryDelete func(context.Context, string, core.Workspace) error
}

func (f temporaryEnvironments) DeleteTemporary(ctx context.Context, name string, w core.Workspace) error {
	return f.temporaryDelete(ctx, name, w)
}
func (f temporaryEnvironments) DeleteRun(ctx context.Context, name, instance string) error {
	if instance != f.createSpec.EphemeralInstance {
		return core.ErrCapabilityStale
	}
	if f.createSpec.TemporaryWorkspace != nil {
		return f.DeleteTemporary(ctx, name, *f.createSpec.TemporaryWorkspace)
	}
	return f.fakeEnvironments.DeleteRun(ctx, name, instance)
}
func TestTemporaryRunRetainsIdentityUntilAllCleanupCompletes(t *testing.T) {
	store := newFakeRunStore()
	ordinary := &fakeEnvironments{}
	cleanupFailed := true
	deleted := false
	var recorded core.Workspace
	env := temporaryEnvironments{ordinary, func(ctx context.Context, name string, w core.Workspace) error {
		marker, ok := store.runs[name]
		if !ok || marker.TemporaryWorkspace == nil || *marker.TemporaryWorkspace != w || ctx.Err() != nil {
			t.Fatal("missing owned, uncanceled cleanup")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("unbounded cleanup")
		}
		recorded = w
		deleted = true
		return nil
	}}
	service := NewWithRecovery(env, store, t.TempDir())
	service.newName = func() (string, error) { return "run-temp", nil }
	service.acquireOwnership = func(string, string, bool) (runOwnershipLock, bool, error) { return &fakeOwnershipLock{}, true, nil }
	service.ConfigureTemporaryWorkspace(func(ctx context.Context, w core.Workspace) error {
		if !deleted || w != recorded {
			t.Fatal("resource cleanup preceded runtime removal")
		}
		if cleanupFailed {
			return errors.New("uncertain resource removal")
		}
		return nil
	})
	result, err := service.Run(context.Background(), Spec{Argv: []string{"true"}})
	if err == nil || result.CleanedUp {
		t.Fatal("failed resource cleanup reported success")
	}
	marker := store.runs["run-temp"]
	if marker.State != core.EphemeralRunCleanupRequired || marker.TemporaryWorkspace == nil || !core.ValidTemporaryWorkspace(*marker.TemporaryWorkspace) {
		t.Fatal("lost recovery ownership")
	}
	if ordinary.createSpec.WorkspacePath != "" || ordinary.createSpec.TemporaryWorkspace == nil || ordinary.createSpec.SkipDefaultResource {
		t.Fatal("default copy or temporary source lost")
	}
	cleanupFailed = false
	if err := service.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.runs) != 0 {
		t.Fatal("completed recovery retained marker")
	}
}
func TestTemporaryRunKeepsResourcesWhileRuntimeAbsenceUncertain(t *testing.T) {
	work, err := core.NewTemporaryWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	env := temporaryEnvironments{&fakeEnvironments{}, func(context.Context, string, core.Workspace) error { return core.ErrIncompatibleState }}
	service := New(env)
	service.ConfigureTemporaryWorkspace(func(context.Context, core.Workspace) error {
		t.Fatal("resource removed while runtime ownership uncertain")
		return nil
	})
	if err := service.cleanupRun(context.Background(), core.EphemeralRun{EnvironmentID: "reused", TemporaryWorkspace: &work}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
}
