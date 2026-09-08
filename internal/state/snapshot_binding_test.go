package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestSnapshotBindingSurvivesRestartAndExactCAS(t *testing.T) {
	s, snap := snapshotCatalogFixture(t)
	ctx := context.Background()
	snap.Components[0].Binding = `{"version":1,"plan":"root-storage"}`
	mustSnapshot(t, s.BeginSnapshot(ctx, snap))
	reopened := NewEnvironmentJSONStore(s.path)
	got, err := reopened.GetSnapshot(ctx, snap.ID)
	mustSnapshot(t, err)
	if got.Components[0] != snap.Components[0] {
		t.Fatal("binding lost")
	}
	stale := got.Components[0]
	stale.Binding = `{"version":1,"plan":"different-storage"}`
	if !errors.Is(reopened.RecordSnapshotComponent(ctx, snap.ID, stale, "created"), core.ErrCapabilityStale) {
		t.Fatal("changed binding accepted")
	}
	mustSnapshot(t, reopened.RecordSnapshotComponent(ctx, snap.ID, got.Components[0], "created"))
	mustSnapshot(t, reopened.PutEnvironment(ctx, snap.Source.Environment))
	got, err = NewEnvironmentJSONStore(s.path).GetSnapshot(ctx, snap.ID)
	mustSnapshot(t, err)
	if got.Components[0].Binding != snap.Components[0].Binding || got.Components[0].State != "created" {
		t.Fatal("ordinary write lost binding")
	}
}
func TestSnapshotBindingSchemaMigration(t *testing.T) {
	for _, binding := range []string{"", `{"version":1}`} {
		t.Run(binding, func(t *testing.T) {
			s, snap := snapshotCatalogFixture(t)
			ctx := context.Background()
			snap.Components[0].Binding = binding
			mustSnapshot(t, s.BeginSnapshot(ctx, snap))
			raw, err := os.ReadFile(s.path)
			mustSnapshot(t, err)
			var data environmentFileState
			mustSnapshot(t, json.Unmarshal(raw, &data))
			data.Version = 5
			raw, err = json.Marshal(data)
			mustSnapshot(t, err)
			mustSnapshot(t, os.WriteFile(s.path, raw, 0600))
			reopened := NewEnvironmentJSONStore(s.path)
			_, err = reopened.GetSnapshot(ctx, snap.ID)
			if binding != "" {
				if !errors.Is(err, core.ErrIncompatibleState) {
					t.Fatal("binding accepted without downgrade barrier", err)
				}
				return
			}
			mustSnapshot(t, err)
			mustSnapshot(t, reopened.MarkSnapshotRecovery(ctx, snap.ID))
			raw, err = os.ReadFile(s.path)
			mustSnapshot(t, err)
			mustSnapshot(t, json.Unmarshal(raw, &data))
			if data.Version != environmentStateVersion || data.Snapshots[snap.ID].State != "recovery-required" {
				t.Fatal("legacy ownership migration lost", data)
			}
		})
	}
}
func TestSnapshotBaseCannotBeOmittedAtReservation(t *testing.T) {
	s, snap := snapshotCatalogFixture(t)
	ctx := context.Background()
	snap.Source.Environment.Base = &core.BaseRef{Name: "custom/base", Revision: "immutable-revision"}
	mustSnapshot(t, s.PutEnvironment(ctx, snap.Source.Environment))
	if !errors.Is(s.BeginSnapshot(ctx, snap), core.ErrInvalidArgument) {
		t.Fatal("Base silently omitted")
	}
	snap.Components = append(snap.Components, core.SnapshotComponent{Role: "base", Owner: strings.Repeat("c", 32), NativeRef: "provider:saved-base", Binding: "base-plan", State: "planned"})
	mustSnapshot(t, s.BeginSnapshot(ctx, snap))
	mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, snap.Components[0], "created"))
	if !errors.Is(s.CommitSnapshot(ctx, snap.ID), core.ErrRecoveryRequired) {
		t.Fatal("incomplete Base published")
	}
}
func TestSnapshotBindingBounds(t *testing.T) {
	for _, binding := range []string{strings.Repeat("x", 16385), string([]byte{255})} {
		s, snap := snapshotCatalogFixture(t)
		snap.Components[0].Binding = binding
		if !errors.Is(s.BeginSnapshot(context.Background(), snap), core.ErrInvalidArgument) {
			t.Fatal("unsafe binding accepted")
		}
	}
}
