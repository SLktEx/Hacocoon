package state

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCacheEmptyPreservesSavedCopiesAndFencesDirectSnapshot(t *testing.T) {
	ctx := context.Background()
	store, saved := savedDataFixture(t)
	lease, err := store.GetWorkspaceLease(ctx, saved.Source.Environment.Name)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := store.BeginEnvironmentResourceClear(ctx, lease, lease.Attachments[0].Resource)
	if err != nil {
		t.Fatal(err)
	}
	reloaded := NewEnvironmentJSONStore(store.path)
	held, err := reloaded.GetSnapshot(ctx, saved.ID)
	if err != nil || !reflect.DeepEqual(held, saved) {
		t.Fatal("saved data changed", held, err)
	}
	if err := reloaded.CheckEnvironmentResourcesIdle(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("reload lost maintenance", err)
	}
	pending := saved
	pending.ID = "snap-" + strings.Repeat("8", 32)
	pending.State = "capturing"
	pending.Components = append([]core.SnapshotComponent(nil), saved.Components...)
	for i := range pending.Components {
		pending.Components[i].State = "planned"
		pending.Components[i].NativeRef += "-next"
	}
	if err := reloaded.BeginSnapshot(ctx, pending); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("direct snapshot bypassed maintenance", err)
	}
	if err := reloaded.CommitEnvironmentResourceClear(ctx, lease, resource); err != nil {
		t.Fatal(err)
	}
	if err := reloaded.BeginSnapshot(ctx, pending); err != nil {
		t.Fatal("completed clear did not permit snapshot", err)
	}
	if _, err := reloaded.BeginEnvironmentResourceClear(ctx, lease, lease.Attachments[0].Resource); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("clear bypassed pending snapshot", err)
	}
}
