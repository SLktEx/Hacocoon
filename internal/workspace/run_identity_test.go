package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

func TestRunReservationAndCleanupCannotSelectAnotherCreation(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "owned-run"}}
	service := New(runtime, store)
	instance, _ := core.NewEnvironmentInstanceID()
	marker := core.EphemeralRun{InstanceID: instance, EnvironmentID: "run-test", State: core.EphemeralRunCreating, CreatedAt: time.Now().UTC()}
	if err := store.PutEphemeralRun(ctx, marker); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	if _, err := service.Create(ctx, core.EnvironmentSpec{Name: marker.EnvironmentID, WorkspacePath: work}); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("ordinary creation claimed run reservation", err)
	}
	env, err := service.Create(ctx, core.EnvironmentSpec{Name: marker.EnvironmentID, WorkspacePath: work, EphemeralInstance: instance})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := store.GetWorkspaceLease(ctx, env.Name)
	if err != nil || !lease.Ephemeral || lease.InstanceID != instance {
		t.Fatal("reservation lost generation", lease, err)
	}
	other, _ := core.NewEnvironmentInstanceID()
	if err := service.DeleteRun(ctx, env.Name, other); !errors.Is(err, core.ErrCapabilityStale) || len(runtime.deleteRefs) != 0 {
		t.Fatal("wrong generation deleted", err)
	}
	if err := store.DeleteEphemeralRun(ctx, env.Name); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("live ownership dropped", err)
	}
	if err := service.DeleteRun(ctx, env.Name, instance); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, core.EnvironmentSpec{Name: env.Name, WorkspacePath: work}); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("unfinished marker did not fence name reuse", err)
	}
	if err := store.DeleteEphemeralRun(ctx, env.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, core.EnvironmentSpec{Name: env.Name, WorkspacePath: work}); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteRun(ctx, env.Name, instance); !errors.Is(err, core.ErrCapabilityStale) || len(runtime.deleteRefs) != 1 {
		t.Fatal("recreated ordinary Environment was deleted", err)
	}
	if _, err := store.GetEnvironment(ctx, env.Name); err != nil {
		t.Fatal("replacement disappeared", err)
	}
}

func TestRunCreationRequiresDurableMatchingReservation(t *testing.T) {
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	runtime := &fakeEnvironmentRuntime{}
	instance, _ := core.NewEnvironmentInstanceID()
	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "run-missing", WorkspacePath: t.TempDir(), EphemeralInstance: instance})
	if !errors.Is(err, core.ErrIncompatibleState) || runtime.createSpec.Name != "" {
		t.Fatal("provider created without run ownership", err)
	}
}
