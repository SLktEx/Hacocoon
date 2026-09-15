package controlapi

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/oci"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
)

type ociAPIBackend struct {
	mu         sync.Mutex
	store      *state.EnvironmentJSONStore
	resources  map[string]core.PersistentResourceRef
	failDelete bool
	deletes    int
}

func (b *ociAPIBackend) Plan(_ context.Context, kind, owner string) (string, error) {
	return "pool/" + kind + "/" + owner, nil
}
func (b *ociAPIBackend) Create(ctx context.Context, r core.PersistentResource) error {
	held, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || held.Ref() != r.Ref() || held.NativeRef != r.NativeRef || held.State != "creating" {
		return core.ErrIncompatibleState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.resources[r.ID]; ok {
		return core.ErrAlreadyExists
	}
	b.resources[r.ID] = r.Ref()
	return nil
}
func (b *ociAPIBackend) Verify(_ context.Context, r core.PersistentResource) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.resources[r.ID] != r.Ref() {
		return core.ErrCapabilityStale
	}
	return nil
}
func (b *ociAPIBackend) CheckDeletion(ctx context.Context, r core.PersistentResource) error {
	// This native fixture has no snapshots or external users. The exact live
	// resource is still verified before the catalog transitions to deleting.
	return b.Verify(ctx, r)
}
func (b *ociAPIBackend) Copy(ctx context.Context, source, target core.PersistentResource) error {
	if err := b.Verify(ctx, source); err != nil {
		return err
	}
	held, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || held.CopySource != source.Ref() {
		return core.ErrCapabilityStale
	}
	return b.Create(ctx, target)
}
func (b *ociAPIBackend) Delete(_ context.Context, r core.PersistentResource) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deletes++
	if b.resources[r.ID] != r.Ref() {
		return core.ErrCapabilityStale
	}
	if b.failDelete {
		return core.ErrStorageUnavailable
	}
	delete(b.resources, r.ID)
	return nil
}

func TestOCIStoreWireKeepsCopiesIndependentAndCleanupRecoveryOwned(t *testing.T) {
	ctx := context.Background()
	catalogPath := filepath.Join(t.TempDir(), "state.json")
	catalog := state.NewEnvironmentJSONStore(catalogPath)
	backend := &ociAPIBackend{store: catalog, resources: map[string]core.PersistentResourceRef{}}
	service := &persistentresource.Service{Store: catalog, Backend: backend}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterOCIStores(s, service); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	created, err := client.OCIStore(ctx, OCIStoreRequest{Operation: "create", ID: "oci:source"})
	if err != nil || len(created.Resources) != 1 || created.Resources[0].State != "ready" || created.Resources[0].Kind != oci.StoreKind {
		t.Fatal(created, err)
	}
	source := created.Resources[0]
	copyResult, err := client.OCIStore(ctx, OCIStoreRequest{Operation: "create", ID: "oci:copy", From: source.ID})
	if err != nil || len(copyResult.Resources) != 1 {
		t.Fatal(copyResult, err)
	}
	copied := copyResult.Resources[0]
	if copied.Owner == source.Owner || copied.NativeRef == source.NativeRef || copied.CopySource != (core.PersistentResourceRef{}) || copied.State != "ready" {
		t.Fatal("copied store retained source ownership", source, copied)
	}
	foreign, err := service.Create(ctx, "cache:other", "build-cache")
	if err != nil {
		t.Fatal(err)
	}
	listed, err := client.OCIStore(ctx, OCIStoreRequest{Operation: "list"})
	if err != nil || len(listed.Resources) != 2 || len(listed.Uses) != 2 {
		t.Fatal("OCI list mixed resource kinds or uses", listed, err)
	}
	for _, r := range listed.Resources {
		if r.ID != source.ID && r.ID != copied.ID {
			t.Fatal("foreign kind exposed", r)
		}
	}
	for _, use := range listed.Uses {
		if (use.Resource != source.Ref() && use.Resource != copied.Ref()) || use.Role != "independent-store" || len(use.Environments) != 0 || len(use.IndependentSnapshots) != 0 || len(use.PendingCopies) != 0 {
			t.Fatal("independent copy acquired another store's references", use)
		}
	}
	inspected, err := client.OCIStore(ctx, OCIStoreRequest{Operation: "inspect", ID: copied.ID})
	if err != nil || len(inspected.Resources) != 1 || !reflect.DeepEqual(inspected.Resources[0], copied) {
		t.Fatal("inspect lost exact copy", inspected, err)
	}
	if _, err := client.OCIStore(ctx, OCIStoreRequest{Operation: "delete", ID: copied.ID, Owner: strings.Repeat("b", 32)}); err == nil {
		t.Fatal("stale owner deleted replacement")
	}
	backend.mu.Lock()
	if backend.deletes != 0 {
		t.Error("stale owner reached provider", backend.deletes)
	}
	backend.failDelete = true
	backend.mu.Unlock()
	request := OCIStoreRequest{Operation: "delete", ID: copied.ID, Owner: copied.Owner}
	var status *control.StatusError
	if _, err := client.OCIStore(ctx, request); !errors.As(err, &status) || status.Code != "recovery_required" {
		t.Fatal("ambiguous cleanup reported success", err)
	}
	retained, err := state.NewEnvironmentJSONStore(catalogPath).GetPersistentResource(ctx, copied.ID)
	if err != nil || retained.Ref() != copied.Ref() || retained.State != "deleting" {
		t.Fatal("cleanup failure released exact owner", retained, err)
	}
	backend.mu.Lock()
	backend.failDelete = false
	backend.mu.Unlock()
	deleted, err := client.OCIStore(ctx, request)
	if err != nil || len(deleted.Resources) != 1 || deleted.Resources[0].State != "deleted" {
		t.Fatal(deleted, err)
	}
	if _, err := catalog.GetPersistentResource(ctx, copied.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("positive absence did not finalize deletion", err)
	}
	for _, want := range []core.PersistentResource{source, foreign} {
		got, err := catalog.GetPersistentResource(ctx, want.ID)
		if err != nil || got.Ref() != want.Ref() || got.State != "ready" {
			t.Fatal("delete touched another resource", got, err)
		}
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.deletes != 2 || len(backend.resources) != 2 || backend.resources[source.ID] != source.Ref() || backend.resources[foreign.ID] != foreign.Ref() {
		t.Fatal("provider cleanup targeted wrong owners", backend.resources, backend.deletes)
	}
}
