package state

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// The catalog's all-or-nothing reservation belongs here. Native import/restore
// and cleanup receipts are exercised by storage/resource's service tests.
func TestSavedEnvironmentInventoryReservesAtomically(t *testing.T) {
	for _, mode := range []string{"complete", "wrong-target", "missing-provenance", "reused-owner", "wrong-snapshot", "source-instance", "incomplete", "copy-source", "changed-origin"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store, saved := savedDataFixture(t)
			lease := core.WorkspaceLease{EnvironmentID: "restored", Owner: "restored", InstanceID: "env-" + strings.Repeat("9", 32), WorkspaceID: "new-work", SourcePath: "managed:new", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), SnapshotSource: saved.ID}
			var plans []core.EnvironmentResourcePlan
			for i, area := range saved.Source.Environment.Attachments {
				resource := core.PersistentResource{ID: fmt.Sprintf("env-data:%032x", 100+i), Owner: fmt.Sprintf("%032x", 200+i), EnvironmentInstance: lease.InstanceID, Kind: area.Origin.Kind, NativeRef: fmt.Sprintf("pool/restored-%d", i), State: "planned", RestoreSource: saved.ID, CreatedAt: lease.AcquiredAt}
				area.Resource = resource.Ref()
				lease.Attachments = append(lease.Attachments, area)
				plans = append(plans, core.EnvironmentResourcePlan{Attachment: area, Resource: resource})
			}
			switch mode {
			case "wrong-target":
				plans[1].Attachment.Target += "/changed"
				lease.Attachments[1] = plans[1].Attachment
			case "missing-provenance":
				plans[1].Resource.RestoreSource = ""
			case "reused-owner":
				plans[1].Resource.Owner = saved.Source.Environment.Attachments[1].Resource.Owner
				plans[1].Attachment.Resource = plans[1].Resource.Ref()
				lease.Attachments[1] = plans[1].Attachment
			case "wrong-snapshot":
				lease.SnapshotSource = "snap-" + strings.Repeat("6", 32)
			case "source-instance":
				lease.InstanceID = saved.Source.InstanceID
			case "incomplete":
				plans = plans[:1]
			case "copy-source":
				plans[1].Resource.CopySource = saved.Source.Environment.Attachments[1].Resource
			case "changed-origin":
				plans[1].Attachment.Origin.Epoch = strings.Repeat("f", 32)
				lease.Attachments[1] = plans[1].Attachment
			}
			before, err := os.ReadFile(store.path)
			if err != nil {
				t.Fatal(err)
			}
			err = store.BeginEnvironmentCreateFromSnapshotWithResources(ctx, lease, saved, plans)
			if mode != "complete" {
				if err == nil {
					t.Fatal("incomplete/foreign snapshot inventory accepted")
				}
				after, readErr := os.ReadFile(store.path)
				if readErr != nil || string(before) != string(after) {
					t.Fatal("failed inventory partially reserved catalog", readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			reloaded := NewEnvironmentJSONStore(store.path)
			held, err := reloaded.GetWorkspaceLease(ctx, lease.EnvironmentID)
			if err != nil || !held.Equal(lease) {
				t.Fatal("reservation did not survive reopen", held, err)
			}
			if err := reloaded.BeginSnapshotDelete(ctx, saved.ID); !errors.Is(err, core.ErrStorageBusy) {
				t.Fatal("reserved restore source released", err)
			}
			for _, plan := range plans {
				r, err := reloaded.BeginEnvironmentResourceMaterialization(ctx, lease, plan.Resource.Ref())
				if err != nil || r.RestoreSource != saved.ID || r.Ref() != plan.Resource.Ref() {
					t.Fatal("restoration lost exact source", r, err)
				}
				r, err = reloaded.RecordEnvironmentResourceCreated(ctx, r)
				if err != nil {
					t.Fatal(err)
				}
				if err := reloaded.CommitPersistentResourceCreate(ctx, r); err != nil {
					t.Fatal(err)
				}
			}
			absent, err := reloaded.PrepareEnvironmentResourceDeletion(ctx, lease)
			if err != nil {
				t.Fatal(err)
			}
			for _, area := range absent.Attachments {
				r, err := reloaded.BeginEnvironmentResourceDelete(ctx, absent.InstanceID, area.Resource)
				if err != nil {
					t.Fatal(err)
				}
				if err := reloaded.FinalizePersistentResourceDelete(ctx, r); err != nil {
					t.Fatal(err)
				}
			}
			if err := reloaded.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); err != nil {
				t.Fatal(err)
			}
			if err := reloaded.BeginSnapshotDelete(ctx, saved.ID); err != nil {
				t.Fatal("completed restore cleanup kept source pinned", err)
			}
		})
	}
}

func TestImportedResourceReservationRequiresExplicitImportMode(t *testing.T) {
	ctx := context.Background()
	store, lease, plans := environmentDataFixture(t, 1)
	plans[0].Resource.ImportPending = true
	if err := store.BeginEnvironmentCreateWithResources(ctx, lease, plans); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginEnvironmentResourceMaterialization(ctx, lease, plans[0].Resource.Ref()); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("import reservation became ordinary create", err)
	}
	r, err := store.BeginEnvironmentResourceImport(ctx, lease, plans[0].Resource.Ref())
	if err != nil || !r.ImportPending || r.State != "creating" {
		t.Fatal("import authority lost", r, err)
	}
	r, err = store.RecordEnvironmentResourceCreated(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitPersistentResourceCreate(ctx, r); err != nil {
		t.Fatal(err)
	}
	held, err := NewEnvironmentJSONStore(store.path).GetPersistentResource(ctx, r.ID)
	if err != nil || held.Ref() != r.Ref() || held.ImportPending || held.State != "ready" {
		t.Fatal("completed import receipt lost", held, err)
	}
}
