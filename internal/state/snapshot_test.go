package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func snapshotCatalogFixture(t *testing.T) (*EnvironmentJSONStore, core.Snapshot) {
	t.Helper()
	s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	ctx := context.Background()
	id, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	lease := core.WorkspaceLease{InstanceID: id, EnvironmentID: "dev", WorkspaceID: "work", SourcePath: "managed:work", AccessMode: core.WorkspaceReadWrite, Owner: "dev", State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	if err := s.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.RuntimeRef = "haco-dev"
	if err := s.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.State = core.WorkspaceLeaseActive
	env := core.Environment{Name: "dev", Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, AccessMode: lease.AccessMode, RuntimeRef: lease.RuntimeRef, CreatedAt: lease.AcquiredAt}
	if err := s.CommitEnvironmentCreate(ctx, env, lease); err != nil {
		t.Fatal(err)
	}
	return s, core.Snapshot{ID: "snap-11111111111111111111111111111111", Source: core.SnapshotSource{Environment: env, InstanceID: id}, State: "capturing", Components: []core.SnapshotComponent{
		{Role: "rootfs", NativeRef: "provider:rootfs-one", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: "planned"},
		{Role: "workspace:main", NativeRef: "provider:workspace-one", Owner: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", State: "planned"},
	}}
}
func mustSnapshot(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotCatalogRecoverySurvivesRestart(t *testing.T) {
	s, snap := snapshotCatalogFixture(t)
	ctx := context.Background()
	mustSnapshot(t, s.BeginSnapshot(ctx, snap))
	mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, snap.Components[0], "created"))
	mustSnapshot(t, s.MarkSnapshotRecovery(ctx, snap.ID))
	s = NewEnvironmentJSONStore(s.path)
	got, err := s.GetSnapshot(ctx, snap.ID)
	mustSnapshot(t, err)
	if got.State != "recovery-required" || got.Components[0].State != "created" || got.Components[1] != snap.Components[1] {
		t.Fatal("lost partial ownership", got)
	}
	if !errors.Is(s.CheckSnapshotIdle(ctx, "dev"), core.ErrRecoveryRequired) {
		t.Fatal("restart released source")
	}
	if !errors.Is(s.FinalizeEnvironmentDelete(ctx, "dev"), core.ErrRecoveryRequired) {
		t.Fatal("deleted reserved source")
	}
	if !errors.Is(s.CommitSnapshot(ctx, snap.ID), core.ErrRecoveryRequired) {
		t.Fatal("partial snapshot published")
	}
	mustSnapshot(t, s.BeginSnapshotDelete(ctx, snap.ID))
	if !errors.Is(s.FinalizeSnapshotDelete(ctx, snap.ID), core.ErrRecoveryRequired) {
		t.Fatal("cleanup attempt released reservation")
	}
	mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, got.Components[0], "absent"))
	if !errors.Is(s.FinalizeSnapshotDelete(ctx, snap.ID), core.ErrRecoveryRequired) {
		t.Fatal("planned target assumed absent")
	}
	mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, got.Components[1], "absent"))
	mustSnapshot(t, s.FinalizeSnapshotDelete(ctx, snap.ID))
	mustSnapshot(t, s.CheckSnapshotIdle(ctx, "dev"))
	if _, err := s.GetSnapshot(ctx, snap.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestSnapshotCatalogPublicationRequiresExactVerifiedComponents(t *testing.T) {
	s, snap := snapshotCatalogFixture(t)
	ctx := context.Background()
	mustSnapshot(t, s.BeginSnapshot(ctx, snap))
	for _, c := range snap.Components {
		if !errors.Is(s.RecordSnapshotComponent(ctx, snap.ID, c, "verified"), core.ErrIncompatibleState) {
			t.Fatal("skipped created receipt")
		}
		wrong := c
		wrong.Owner = "cccccccccccccccccccccccccccccccc"
		if !errors.Is(s.RecordSnapshotComponent(ctx, snap.ID, wrong, "created"), core.ErrCapabilityStale) {
			t.Fatal("owner drift accepted")
		}
		mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, c, "created"))
		if !errors.Is(s.RecordSnapshotComponent(ctx, snap.ID, c, "created"), core.ErrCapabilityStale) {
			t.Fatal("stale state accepted")
		}
		c.State = "created"
		mustSnapshot(t, s.RecordSnapshotComponent(ctx, snap.ID, c, "verified"))
	}
	mustSnapshot(t, s.CommitSnapshot(ctx, snap.ID))
	mustSnapshot(t, s.CheckSnapshotIdle(ctx, "dev"))
	got, err := NewEnvironmentJSONStore(s.path).GetSnapshot(ctx, snap.ID)
	mustSnapshot(t, err)
	if got.State != "ready" {
		t.Fatal(got.State)
	}
	collision := snap
	collision.ID = "snap-22222222222222222222222222222222"
	if !errors.Is(s.BeginSnapshot(ctx, collision), core.ErrAlreadyExists) {
		t.Fatal("saved targets reused by another snapshot")
	}
	mustSnapshot(t, s.FinalizeEnvironmentDelete(ctx, "dev"))
	after, err := s.GetSnapshot(ctx, snap.ID)
	mustSnapshot(t, err)
	if !reflect.DeepEqual(got, after) {
		t.Fatal("ordinary catalog write lost snapshot")
	}
}
func TestSnapshotCatalogRejectsInvalidPlans(t *testing.T) {
	for _, mode := range []string{"source-drift", "instance-drift", "missing-oci", "duplicate-role", "duplicate-target", "missing-workspace", "bad-owner", "bad-state", "control-role"} {
		t.Run(mode, func(t *testing.T) {
			s, snap := snapshotCatalogFixture(t)
			switch mode {
			case "source-drift":
				snap.Source.Environment.RuntimeRef = "replaced"
			case "instance-drift":
				snap.Source.InstanceID = "env-22222222222222222222222222222222"
			case "missing-oci":
				snap.Source.Environment.PersistentResource = core.PersistentResourceRef{ID: "oci:one", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			case "duplicate-role":
				snap.Components[1].Role = "rootfs"
			case "duplicate-target":
				snap.Components[1].NativeRef = snap.Components[0].NativeRef
			case "missing-workspace":
				snap.Components = snap.Components[:1]
			case "bad-owner":
				snap.Components[1].Owner = "unknown"
			case "bad-state":
				snap.Components[1].State = "verified"
			case "control-role":
				snap.Components[1].Role = "workspace:x\n"
			}
			if err := s.BeginSnapshot(context.Background(), snap); err == nil {
				t.Fatal("unsafe plan accepted")
			}
			mustSnapshot(t, s.CheckSnapshotIdle(context.Background(), "dev"))
		})
	}
}
func TestSnapshotCatalogConcurrentReservation(t *testing.T) {
	s, snap := snapshotCatalogFixture(t)
	ctx := context.Background()
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			candidate := snap
			if i == 1 {
				candidate.ID = "snap-22222222222222222222222222222222"
			}
			errs <- NewEnvironmentJSONStore(s.path).BeginSnapshot(ctx, candidate)
		}(i)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("concurrent captures", success)
	}
}
func TestSnapshotCatalogSchemaAndCancellation(t *testing.T) {
	s, snap := snapshotCatalogFixture(t)
	ctx := context.Background()
	raw, err := os.ReadFile(s.path)
	mustSnapshot(t, err)
	var data map[string]any
	mustSnapshot(t, json.Unmarshal(raw, &data))
	data["version"] = 4
	raw, err = json.Marshal(data)
	mustSnapshot(t, err)
	mustSnapshot(t, os.WriteFile(s.path, raw, 0600))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if !errors.Is(s.BeginSnapshot(canceled, snap), context.Canceled) {
		t.Fatal("canceled reservation persisted")
	}
	mustSnapshot(t, s.BeginSnapshot(ctx, snap))
	raw, err = os.ReadFile(s.path)
	mustSnapshot(t, err)
	mustSnapshot(t, json.Unmarshal(raw, &data))
	if data["version"] != float64(environmentStateVersion) {
		t.Fatal("missing downgrade barrier")
	}
	for _, version := range []int{4, environmentStateVersion + 1} {
		data["version"] = version
		raw, err = json.Marshal(data)
		mustSnapshot(t, err)
		mustSnapshot(t, os.WriteFile(s.path, raw, 0600))
		if err := s.CheckSnapshotIdle(ctx, "dev"); err == nil {
			t.Fatal("unsafe schema accepted", version)
		}
	}
}
