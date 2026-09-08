package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func restoreFixture(t *testing.T) (*EnvironmentJSONStore, core.SnapshotRestore) {
	t.Helper()
	s, saved := snapshotCatalogFixture(t)
	ctx := context.Background()
	publish := func(snap core.Snapshot) core.Snapshot {
		for i := range snap.Components {
			snap.Components[i].Binding = `{"fixture":1}`
		}
		mustSnapshot(t, s.BeginSnapshot(ctx, snap))
		for i, c := range snap.Components {
			mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, c, "created"))
			c.State = "created"
			mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, c, "verified"))
			c.State = "verified"
			snap.Components[i] = c
		}
		mustSnapshot(t, s.CommitSnapshot(ctx, snap.ID))
		snap.State = "ready"
		return snap
	}
	saved = publish(saved)
	before := saved
	before.ID = "snap-" + strings.Repeat("2", 32)
	before.State = "capturing"
	before.Components = append([]core.SnapshotComponent(nil), saved.Components...)
	for i := range before.Components {
		before.Components[i].NativeRef += "-backup"
		before.Components[i].Owner = strings.Repeat(string(rune('c'+i)), 32)
		before.Components[i].State = "planned"
	}
	before = publish(before)
	op := core.SnapshotRestore{ID: "restore-" + strings.Repeat("3", 32), Saved: saved, Before: before, State: "preparing", Components: append([]core.SnapshotComponent(nil), saved.Components...)}
	for i := range op.Components {
		op.Components[i].NativeRef += "-restored"
		op.Components[i].Owner = strings.Repeat(string(rune('e'+i)), 32)
		op.Components[i].State = "planned"
	}
	return s, op
}

func TestRestorePreparationKeepsBothSnapshotsAndCurrentEnvironmentAcrossRestart(t *testing.T) {
	s, op := restoreFixture(t)
	ctx := context.Background()
	mustSnapshot(t, s.BeginSnapshotRestore(ctx, op))
	mustSnapshot(t, s.RecordRestoreComponent(ctx, op.ID, op.Components[0], "created"))
	op.Components[0].State = "created"
	mustSnapshot(t, s.MarkRestoreRecovery(ctx, op.ID))
	op.State = "recovery-required"
	s = NewEnvironmentJSONStore(s.path)
	got, err := s.GetSnapshotRestore(ctx, op.ID)
	mustSnapshot(t, err)
	if !reflect.DeepEqual(got, op) {
		t.Fatal("lost interrupted ownership")
	}
	for _, id := range []string{op.Saved.ID, op.Before.ID} {
		if !errors.Is(s.BeginSnapshotDelete(ctx, id), core.ErrStorageBusy) {
			t.Fatal("reserved snapshot deleted")
		}
	}
	if !errors.Is(s.CheckSnapshotIdle(ctx, "dev"), core.ErrRecoveryRequired) || !errors.Is(s.FinalizeEnvironmentDelete(ctx, "dev"), core.ErrRecoveryRequired) {
		t.Fatal("target released during restore")
	}
	if !errors.Is(s.CommitRestorePreparation(ctx, op.ID), core.ErrRecoveryRequired) {
		t.Fatal("partial preparation published")
	}
	drift := op.Components[0]
	drift.Binding = `{"fixture":2}`
	if !errors.Is(s.RecordRestoreComponent(ctx, op.ID, drift, "verified"), core.ErrCapabilityStale) {
		t.Fatal("binding drift accepted")
	}
	mustSnapshot(t, s.BeginRestoreCleanup(ctx, op.ID))
	if !errors.Is(s.FinalizeRestoreCleanup(ctx, op.ID), core.ErrRecoveryRequired) {
		t.Fatal("cleanup attempt released ownership")
	}
	for _, c := range op.Components {
		mustSnapshot(t, s.RecordRestoreComponent(ctx, op.ID, c, "absent"))
	}
	mustSnapshot(t, s.FinalizeRestoreCleanup(ctx, op.ID))
	mustSnapshot(t, s.CheckSnapshotIdle(ctx, "dev"))
	for _, id := range []string{op.Saved.ID, op.Before.ID} {
		snap, err := s.GetSnapshot(ctx, id)
		mustSnapshot(t, err)
		if snap.State != "ready" {
			t.Fatal("preserved snapshot mutated")
		}
	}
	env, err := s.GetEnvironment(ctx, "dev")
	mustSnapshot(t, err)
	if !reflect.DeepEqual(env, op.Before.Source.Environment) {
		t.Fatal("preparation changed current environment")
	}
}

func TestRestorePreparationRequiresEveryReceiptBeforePrepared(t *testing.T) {
	s, op := restoreFixture(t)
	ctx := context.Background()
	mustSnapshot(t, s.BeginSnapshotRestore(ctx, op))
	for _, c := range op.Components {
		if s.RecordRestoreComponent(ctx, op.ID, c, "verified") == nil {
			t.Fatal("skipped create receipt")
		}
		mustSnapshot(t, s.RecordRestoreComponent(ctx, op.ID, c, "created"))
		c.State = "created"
		mustSnapshot(t, s.RecordRestoreComponent(ctx, op.ID, c, "verified"))
	}
	mustSnapshot(t, s.CommitRestorePreparation(ctx, op.ID))
	got, err := s.GetSnapshotRestore(ctx, op.ID)
	mustSnapshot(t, err)
	if got.State != "prepared" || s.CheckSnapshotIdle(ctx, "dev") == nil {
		t.Fatal("preparation published runnable environment")
	}
}

func TestRestoreReservationRacesSnapshotDeletionAtomically(t *testing.T) {
	for attempt := 0; attempt < 12; attempt++ {
		s, op := restoreFixture(t)
		ctx := context.Background()
		var wg sync.WaitGroup
		var a, b error
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; a = NewEnvironmentJSONStore(s.path).BeginSnapshotRestore(ctx, op) }()
		go func() {
			defer wg.Done()
			<-start
			b = NewEnvironmentJSONStore(s.path).BeginSnapshotDelete(ctx, op.Saved.ID)
		}()
		close(start)
		wg.Wait()
		if (a == nil) == (b == nil) {
			t.Fatalf("nonexclusive outcomes %v %v", a, b)
		}
	}
}

func TestRestoreCatalogRejectsDowngradeAndOwnershipDrift(t *testing.T) {
	for _, mode := range []string{"schema7", "snapshot-drift", "target-owner", "omitted-component", "duplicate-target", "target-current"} {
		t.Run(mode, func(t *testing.T) {
			s, op := restoreFixture(t)
			ctx := context.Background()
			mustSnapshot(t, s.BeginSnapshotRestore(ctx, op))
			raw, err := os.ReadFile(s.path)
			mustSnapshot(t, err)
			var d environmentFileState
			mustSnapshot(t, json.Unmarshal(raw, &d))
			switch mode {
			case "schema7":
				d.Version = 7
			case "snapshot-drift":
				snap := d.Snapshots[op.Saved.ID]
				snap.Components[0].Binding = "drift"
				d.Snapshots[snap.ID] = snap
			case "target-owner":
				op.Components[0].Owner = op.Saved.Components[0].Owner
				d.Restores[op.ID] = op
			case "omitted-component":
				op.Components = op.Components[:1]
				d.Restores[op.ID] = op
			case "duplicate-target":
				copy := op
				copy.ID = "restore-" + strings.Repeat("4", 32)
				d.Restores[copy.ID] = copy
			case "target-current":
				delete(d.Environments, "dev")
			}
			raw, err = json.Marshal(d)
			mustSnapshot(t, err)
			mustSnapshot(t, os.WriteFile(s.path, raw, 0600))
			if _, err := NewEnvironmentJSONStore(s.path).GetSnapshotRestore(ctx, op.ID); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("invalid catalog read", err)
			}
		})
	}
}
