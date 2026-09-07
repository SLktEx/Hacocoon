package persistentresource_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type copyBackend struct {
	backend
	test   *testing.T
	copies int
}

func (b *copyBackend) Plan(_ context.Context, _, owner string) (string, error) {
	return "pool/" + owner, nil
}
func (b *copyBackend) Copy(ctx context.Context, source, target core.PersistentResource) error {
	b.copies++
	// Reopen the store: the reservation must survive process loss/restart.
	saved, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || saved != target || saved.CopySource != source.Ref() {
		b.test.Fatalf("copy started without durable ownership: %+v %v", saved, err)
	}
	if _, err := b.store.BeginPersistentResourceDelete(ctx, source.ID); !errors.Is(err, core.ErrStorageBusy) {
		b.test.Fatalf("source deletion during copy: %v", err)
	}
	if _, err := b.store.BeginPersistentResourceDelete(ctx, target.ID); !errors.Is(err, core.ErrRecoveryRequired) {
		b.test.Fatalf("target deletion during copy: %v", err)
	}
	lease := core.WorkspaceLease{EnvironmentID: "copy-attach", Owner: "owner", WorkspaceID: "work", SourcePath: "/work", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: source.Ref()}
	if err := b.store.BeginEnvironmentCreate(ctx, lease); !errors.Is(err, core.ErrStorageBusy) {
		b.test.Fatalf("source attached during copy: %v", err)
	}
	if b.fail == "copy" {
		return errors.New("lost provider reply")
	}
	return nil
}
func TestCopyReservesExactSourceUntilVerifiedCommit(t *testing.T) {
	ctx := context.Background()
	for _, failure := range []string{"", "copy", "verify"} {
		t.Run(failure, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			store := state.NewEnvironmentJSONStore(path)
			b := &copyBackend{backend: backend{store: store}, test: t}
			svc := &persistentresource.Service{Store: store, Backend: b}
			source, err := svc.Create(ctx, "oci:source", "oci-containerd")
			if err != nil {
				t.Fatal(err)
			}
			b.fail = failure
			target, err := svc.Copy(ctx, "oci:target", source.Kind, source.ID)
			reopened := state.NewEnvironmentJSONStore(path)
			if failure != "" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("copy error: %v", err)
				}
				held, e := reopened.GetPersistentResource(ctx, target.ID)
				if e != nil || held.CopySource != source.Ref() || held.State != "creating" {
					t.Fatalf("lost copy reservation: %+v %v", held, e)
				}
				if err := svc.Delete(ctx, source.ID); !errors.Is(err, core.ErrStorageBusy) {
					t.Fatalf("source released after failure: %v", err)
				}
				if err := svc.Delete(ctx, target.ID); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("ambiguous target removed: %v", err)
				}
			} else {
				if err != nil || target.State != "ready" || target.CopySource != (core.PersistentResourceRef{}) || target.Owner == source.Owner || target.NativeRef == source.NativeRef {
					t.Fatalf("copy: %+v %v", target, err)
				}
				if err := svc.Delete(ctx, source.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := reopened.GetPersistentResource(ctx, target.ID); err != nil {
					t.Fatal("source delete removed copy", err)
				}
				if err := svc.Delete(ctx, target.ID); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestCopyRejectsAttachedSourceAndInvalidTargetsBeforeProvider(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	b := &copyBackend{backend: backend{store: store}, test: t}
	svc := &persistentresource.Service{Store: store, Backend: b}
	source, err := svc.Create(ctx, "oci:source", "oci-containerd")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{source.ID, "oci:../escape", "oci:--flag", "oci:" + strings.Repeat("a", 41)} {
		if _, err := svc.Copy(ctx, id, source.Kind, source.ID); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	lease := core.WorkspaceLease{EnvironmentID: "dev", Owner: "owner", WorkspaceID: "work", SourcePath: "/work", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: source.Ref()}
	if err := store.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Copy(ctx, "oci:target", source.Kind, source.ID); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatalf("copied active source: %v", err)
	}
	if b.copies != 0 {
		t.Fatal("rejected copy reached provider")
	}
	if _, err := store.GetPersistentResource(ctx, "oci:target"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("rejected copy left target", err)
	}
}
