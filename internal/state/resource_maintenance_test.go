package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// A scratch path alone must never relax the ordinary Workspace association.
// Maintenance requires the durable run record for this exact scratch identity;
// the Store owner and the existing exclusive lease remain authoritative.
func TestResourceMaintenanceLeaseRequiresExactRunAndRetainsData(t *testing.T) {
	for _, mode := range []string{"valid", "missing-run", "other-work", "other-run", "active-run", "host-path", "readonly", "source", "stale-owner"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			work, err := core.NewTemporaryWorkspace()
			if err != nil {
				t.Fatal(err)
			}
			r := core.PersistentResource{ID: "oci:retained", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", NativeRef: "pool/retained", State: "creating", WorkspaceID: "original", CreatedAt: time.Now().UTC()}
			if mode == "source" {
				r.SourceOnly = true
				r.WorkspaceID = ""
			}
			if err := s.BeginPersistentResourceCreate(ctx, r); err != nil {
				t.Fatal(err)
			}
			if err := s.CommitPersistentResourceCreate(ctx, r); err != nil {
				t.Fatal(err)
			}
			run := core.EphemeralRun{EnvironmentID: "maintenance", TemporaryWorkspace: &work, State: core.EphemeralRunCreating, CreatedAt: time.Now().UTC()}
			switch mode {
			case "other-work":
				other, _ := core.NewTemporaryWorkspace()
				run.TemporaryWorkspace = &other
			case "other-run":
				run.EnvironmentID = "other"
			case "active-run":
				run.State = core.EphemeralRunActive
			}
			if mode != "missing-run" {
				if err := s.PutEphemeralRun(ctx, run); err != nil {
					t.Fatal(err)
				}
			}
			lease := core.WorkspaceLease{WorkspaceID: work.ID, SourcePath: work.Path, EnvironmentID: "maintenance", Owner: "maintenance", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: r.Ref()}
			switch mode {
			case "host-path":
				lease.SourcePath = "/original"
			case "readonly":
				lease.AccessMode = core.WorkspaceReadOnly
			case "stale-owner":
				lease.PersistentResource.Owner = strings.Repeat("b", 32)
			}
			err = s.BeginEnvironmentCreate(ctx, lease)
			if mode != "valid" {
				if err == nil {
					t.Fatal("unsafe maintenance reservation accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			run.State = core.EphemeralRunActive
			if err := s.PutEphemeralRun(ctx, run); err != nil {
				t.Fatal(err)
			}
			if err := s.DeleteEphemeralRun(ctx, run.EnvironmentID); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("removed leased evidence: %v", err)
			}
			changed := run
			otherWork, _ := core.NewTemporaryWorkspace()
			changed.TemporaryWorkspace = &otherWork
			if err := s.PutEphemeralRun(ctx, changed); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("replaced leased evidence: %v", err)
			}
			run.State = core.EphemeralRunCleanupRequired
			if err := s.PutEphemeralRun(ctx, run); err != nil {
				t.Fatal(err)
			}
			// Ordinary original-Workspace use and explicit Store deletion stay excluded.
			other := lease
			other.EnvironmentID = "ordinary"
			other.Owner = "ordinary"
			other.WorkspaceID = "original"
			other.SourcePath = "/original"
			if err := s.BeginEnvironmentCreate(ctx, other); !errors.Is(err, core.ErrStorageBusy) {
				t.Fatalf("shared Store: %v", err)
			}
			if _, err := s.BeginPersistentResourceDelete(ctx, r.ID); !errors.Is(err, core.ErrStorageBusy) {
				t.Fatalf("deleted leased Store: %v", err)
			}
			lease.RuntimeRef = "haco-maintenance"
			if err := s.RecordEnvironmentRuntime(ctx, lease); err != nil {
				t.Fatal(err)
			}
			lease.State = core.WorkspaceLeaseCleanupRequired
			if err := s.MarkEnvironmentRecoveryRequired(ctx, lease); err != nil {
				t.Fatal(err)
			}
			if _, err := s.BeginPersistentResourceDelete(ctx, r.ID); !errors.Is(err, core.ErrStorageBusy) {
				t.Fatalf("lost ambiguous cleanup reservation: %v", err)
			}
			// Only the canonical caller may finalize after positive native absence.
			if err := s.FinalizeEnvironmentDelete(ctx, "maintenance"); err != nil {
				t.Fatal(err)
			}
			retained, err := s.GetPersistentResource(ctx, r.ID)
			if err != nil || retained.Ref() != r.Ref() || retained.State != "ready" || retained.WorkspaceID != "original" {
				t.Fatalf("retained Store changed: %+v %v", retained, err)
			}
		})
	}
}
