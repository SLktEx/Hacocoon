package persistentresource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Store interface {
	GetPersistentResource(context.Context, string) (core.PersistentResource, error)
	ListPersistentResources(context.Context) ([]core.PersistentResource, error)
	BeginPersistentResourceCreate(context.Context, core.PersistentResource) error
	CommitPersistentResourceCreate(context.Context, core.PersistentResource) error
	BeginPersistentResourceDelete(context.Context, string) (core.PersistentResource, error)
	FinalizePersistentResourceDelete(context.Context, core.PersistentResource) error
}

type Backend interface {
	Plan(context.Context, string, string) (string, error)
	Create(context.Context, core.PersistentResource) error
	Verify(context.Context, core.PersistentResource) error
	// Delete succeeds only after positively observing absence of the owned resource.
	Delete(context.Context, core.PersistentResource) error
}

type Service struct {
	Store   Store
	Backend Backend
}

func (s *Service) Create(ctx context.Context, id, kind string) (core.PersistentResource, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return core.PersistentResource{}, err
	}
	r := core.PersistentResource{ID: id, Kind: kind, Owner: hex.EncodeToString(nonce[:]), State: "creating", CreatedAt: time.Now().UTC()}
	if !core.ValidPersistentResourceRef(r.Ref()) {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	native, err := s.Backend.Plan(ctx, kind, r.Owner)
	if err != nil {
		return core.PersistentResource{}, err
	}
	r.NativeRef = native
	// Record the exact provider identity before issuing a fallible create request.
	if err := s.Store.BeginPersistentResourceCreate(ctx, r); err != nil {
		return core.PersistentResource{}, err
	}
	if err := s.Backend.Create(ctx, r); err != nil {
		return r, fmt.Errorf("resource creation incomplete; inspect or delete %s: %w: %w", id, core.ErrRecoveryRequired, err)
	}
	if err := s.Backend.Verify(ctx, r); err != nil {
		return r, fmt.Errorf("resource verification failed: %w: %w", core.ErrRecoveryRequired, err)
	}
	if err := s.Store.CommitPersistentResourceCreate(ctx, r); err != nil {
		return r, err
	}
	r.State = "ready"
	return r, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	r, err := s.Store.BeginPersistentResourceDelete(ctx, id)
	if err != nil {
		return err
	}
	if err := s.Backend.Delete(ctx, r); err != nil {
		return fmt.Errorf("resource retained for recovery; retry explicit delete: %w: %w", core.ErrRecoveryRequired, err)
	}
	return s.Store.FinalizePersistentResourceDelete(ctx, r)
}
