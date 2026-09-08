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

func TestPersistentResourceLeaseSurvivesRecoveryAndEnvironmentDeletionRetainsStore(t *testing.T) {
	ctx := context.Background()
	s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	resource := core.PersistentResource{ID: "oci:one", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", NativeRef: "pool/one", State: "creating", CreatedAt: time.Now().UTC()}
	if err := s.BeginPersistentResourceCreate(ctx, resource); err != nil {
		t.Fatal(err)
	}
	lease := core.WorkspaceLease{WorkspaceID: "ws-a", SourcePath: "/a", EnvironmentID: "a", Owner: "a", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: resource.Ref()}
	if err := s.BeginEnvironmentCreate(ctx, lease); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("creating store attached: %v", err)
	}
	if err := s.CommitPersistentResourceCreate(ctx, resource); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	other := lease
	other.EnvironmentID = "b"
	other.Owner = "b"
	other.WorkspaceID = "ws-b"
	other.SourcePath = "/b"
	if err := s.BeginEnvironmentCreate(ctx, other); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatalf("shared RW: %v", err)
	}
	if _, err := s.BeginPersistentResourceDelete(ctx, resource.ID); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatalf("deleted reserved store: %v", err)
	}
	lease.RuntimeRef = "haco-a"
	if err := s.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	changed := lease
	changed.PersistentResource.Owner = strings.Repeat("b", 32)
	if err := s.RecordEnvironmentRuntime(ctx, changed); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("changed owner: %v", err)
	}
	lease.State = core.WorkspaceLeaseCleanupRequired
	if err := s.MarkEnvironmentRecoveryRequired(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginPersistentResourceDelete(ctx, resource.ID); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatalf("lost recovery lease: %v", err)
	}
	// This transition is called only after the runtime has positively disappeared.
	if err := s.FinalizeEnvironmentDelete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	retained, err := s.GetPersistentResource(ctx, resource.ID)
	if err != nil || retained.State != "ready" {
		t.Fatalf("store deleted with environment: %+v %v", retained, err)
	}
	if err := s.BeginEnvironmentCreate(ctx, other); err != nil {
		t.Fatalf("reattach: %v", err)
	}
	if err := s.FinalizeEnvironmentDelete(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	deleting, err := s.BeginPersistentResourceDelete(ctx, resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BeginEnvironmentCreate(ctx, other); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("attach while deleting: %v", err)
	}
	if err := s.FinalizePersistentResourceDelete(ctx, deleting); err != nil {
		t.Fatal(err)
	}
}

func TestPersistentCatalogCorruptionFailsClosed(t *testing.T) {
	ctx := context.Background()
	s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	r := core.PersistentResource{ID: "oci:test", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", NativeRef: "pool/test", State: "creating", CreatedAt: time.Now().UTC()}
	if err := s.BeginPersistentResourceCreate(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.CommitPersistentResourceCreate(ctx, r); err != nil {
		t.Fatal(err)
	}
	// Simulate corrupted metadata rather than silently adopting a different owner.
	if err := s.resourceTransaction(func(d *environmentFileState) error {
		x := d.PersistentResources[r.ID]
		x.Owner = "invalid"
		d.PersistentResources[r.ID] = x
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListPersistentResources(ctx); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("corrupt ownership accepted: %v", err)
	}
}

func TestPublicationAndWorkspaceAssociationsCannotBeBypassedByDirectLease(t *testing.T) {
	for _, sourceOnly := range []bool{true, false} {
		ctx := context.Background()
		s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
		r := core.PersistentResource{ID: "oci:bound", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", NativeRef: "pool/bound", State: "creating", CreatedAt: time.Now().UTC(), SourceOnly: sourceOnly}
		if !sourceOnly {
			r.WorkspaceID = "original"
		}
		if err := s.BeginPersistentResourceCreate(ctx, r); err != nil {
			t.Fatal(err)
		}
		if err := s.CommitPersistentResourceCreate(ctx, r); err != nil {
			t.Fatal(err)
		}
		lease := core.WorkspaceLease{WorkspaceID: "other", SourcePath: "/other", EnvironmentID: "other", Owner: "other", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: r.Ref()}
		if err := s.BeginEnvironmentCreate(ctx, lease); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatalf("source=%t accepted direct attachment: %v", sourceOnly, err)
		}
	}
}
