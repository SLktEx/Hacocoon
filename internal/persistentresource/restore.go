package persistentresource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"time"
)

type savedResourceBackend interface {
	PlanSavedResource(context.Context, core.Snapshot, string) (string, string, error)
	CreateSavedResource(context.Context, core.Snapshot, core.PersistentResource) error
}
type savedResourceCatalog interface {
	BeginSnapshotResourceRestore(context.Context, core.Snapshot, core.PersistentResource) error
	RecordPersistentResourceCreated(context.Context, core.PersistentResource) error
	BeginPersistentResourceDeleteOwned(context.Context, core.PersistentResource) (core.PersistentResource, error)
}

// RestoreSnapshot registers an independent Store, holding the saved aggregate
// only until positive copy completion/publication or exact-owned cleanup.
func (s *Service) RestoreSnapshot(ctx context.Context, id string, saved core.Snapshot, work core.WorkspaceID) (core.PersistentResource, error) {
	backend, ok := s.Backend.(savedResourceBackend)
	if !ok {
		return core.PersistentResource{}, core.ErrUnsupported
	}
	catalog, ok := s.Store.(savedResourceCatalog)
	if !ok {
		return core.PersistentResource{}, core.ErrUnsupported
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return core.PersistentResource{}, err
	}
	r := core.PersistentResource{ID: id, Owner: hex.EncodeToString(nonce[:]), WorkspaceID: work, RestoreSource: saved.ID, State: "creating", CreatedAt: time.Now().UTC()}
	if !core.ValidPersistentResourceRef(r.Ref()) || saved.ID == "" || saved.State != "ready" {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	var err error
	r.Kind, r.NativeRef, err = backend.PlanSavedResource(ctx, saved, r.Owner)
	if err != nil {
		return core.PersistentResource{}, err
	}
	if err := catalog.BeginSnapshotResourceRestore(ctx, saved, r); err != nil {
		return core.PersistentResource{}, err
	}
	fail := func(cause error) (core.PersistentResource, error) {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		owned, cleanupErr := catalog.BeginPersistentResourceDeleteOwned(cleanup, r)
		if cleanupErr == nil {
			cleanupErr = s.Backend.Delete(cleanup, owned)
		}
		if cleanupErr == nil {
			cleanupErr = s.Store.FinalizePersistentResourceDelete(cleanup, owned)
		}
		if cleanupErr != nil {
			return r, fmt.Errorf("restored Store %s cleanup incomplete: %w", id, errors.Join(cause, cleanupErr, core.ErrRecoveryRequired))
		}
		if errors.Is(cause, core.ErrRecoveryRequired) {
			cause = core.ErrRuntimeUnavailable
		}
		return core.PersistentResource{}, fmt.Errorf("restored Store %s failed; owned copy removed: %w", id, cause)
	}
	if err := backend.CreateSavedResource(ctx, saved, r); err != nil {
		return fail(err)
	}
	if err := catalog.RecordPersistentResourceCreated(ctx, r); err != nil {
		return fail(err)
	}
	r.State = "created"
	if err := s.Backend.Verify(ctx, r); err != nil {
		return fail(err)
	}
	if err := s.Store.CommitPersistentResourceCreate(ctx, r); err != nil {
		return fail(err)
	}
	r.State = "ready"
	r.RestoreSource = ""
	return r, nil
}
