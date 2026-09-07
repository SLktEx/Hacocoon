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
	BeginPersistentResourceCopy(context.Context, core.PersistentResource, core.PersistentResource) error
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
	return s.create(ctx, id, kind, false, nil)
}

// PublishSource records ownership before preparing content and publishes only
// after preparation and provider verification. An unfinished source cannot be
// copied or attached, so callers cannot accidentally expose partial content.
func (s *Service) PublishSource(ctx context.Context, id, kind string, prepare func(context.Context, core.PersistentResource) error) (core.PersistentResource, error) {
	if prepare == nil {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	return s.create(ctx, id, kind, true, prepare)
}
func (s *Service) create(ctx context.Context, id, kind string, sourceOnly bool, prepare func(context.Context, core.PersistentResource) error) (core.PersistentResource, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return core.PersistentResource{}, err
	}
	r := core.PersistentResource{SourceOnly: sourceOnly, ID: id, Kind: kind, Owner: hex.EncodeToString(nonce[:]), State: "creating", CreatedAt: time.Now().UTC()}
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
	if prepare != nil {
		if err := prepare(ctx, r); err != nil {
			return r, fmt.Errorf("source publication incomplete; inspect %s: %w: %w", id, core.ErrRecoveryRequired, err)
		}
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

// Copy creates an independent offline resource without exposing its contents to
// Core or the trusted Host. Backends may opt in; no OCI runtime is required.
func (s *Service) Copy(ctx context.Context, id, kind, sourceID string) (core.PersistentResource, error) {
	return s.copy(ctx, id, kind, sourceID, "")
}

// CopyForWorkspace durably binds an automatic copy to its Workspace before
// the provider is called. Retries can never adopt an unrelated same-name Store.
func (s *Service) CopyForWorkspace(ctx context.Context, id, kind, sourceID string, workspaceID core.WorkspaceID) (core.PersistentResource, error) {
	if workspaceID == "" {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	return s.copy(ctx, id, kind, sourceID, workspaceID)
}

func (s *Service) copy(ctx context.Context, id, kind, sourceID string, workspaceID core.WorkspaceID) (core.PersistentResource, error) {
	copier, ok := s.Backend.(interface {
		Copy(context.Context, core.PersistentResource, core.PersistentResource) error
	})
	if !ok {
		return core.PersistentResource{}, core.ErrUnsupported
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return core.PersistentResource{}, err
	}
	target := core.PersistentResource{WorkspaceID: workspaceID, ID: id, Kind: kind, Owner: hex.EncodeToString(nonce[:]), State: "creating", CreatedAt: time.Now().UTC()}
	if !core.ValidPersistentResourceRef(target.Ref()) || id == sourceID {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	source, err := s.Store.GetPersistentResource(ctx, sourceID)
	if err != nil {
		return core.PersistentResource{}, err
	}
	if source.Kind != kind || source.State != "ready" {
		return core.PersistentResource{}, core.ErrIncompatibleState
	}
	target.CopySource = source.Ref()
	target.NativeRef, err = s.Backend.Plan(ctx, kind, target.Owner)
	if err != nil {
		return core.PersistentResource{}, err
	}
	if err := s.Store.BeginPersistentResourceCopy(ctx, source, target); err != nil {
		return core.PersistentResource{}, err
	}
	incomplete := func(err error) (core.PersistentResource, error) {
		return target, fmt.Errorf("copy incomplete; source and destination retained for recovery; inspect %s: %w: %w", id, core.ErrRecoveryRequired, err)
	}
	if err := copier.Copy(ctx, source, target); err != nil {
		return incomplete(err)
	}
	if err := s.Backend.Verify(ctx, target); err != nil {
		return incomplete(err)
	}
	if err := s.Store.CommitPersistentResourceCreate(ctx, target); err != nil {
		return incomplete(err)
	}
	target.State = "ready"
	target.CopySource = core.PersistentResourceRef{}
	return target, nil
}
