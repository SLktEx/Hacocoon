package oci

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
)

// PublishedStoreID is outside the user-managed oci: namespace. Only a trusted
// image publisher should create this source; guest Stores never return to Host.
const PublishedStoreID = "oci-source:host"

type WorkspaceStores struct{ Resources *persistentresource.Service }

// Resolve runs under the canonical Workspace lock. Existing guest changes win
// over newer publications; a failed copy remains visible and recovery-required.
func (s WorkspaceStores) Resolve(ctx context.Context, work core.Workspace) (core.PersistentResource, error) {
	if s.Resources == nil || work.ID == "" {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	sum := sha256.Sum256([]byte(work.ID))
	id := fmt.Sprintf("oci:auto-%x", sum[:16])
	existing, err := s.Resources.Store.GetPersistentResource(ctx, id)
	if err == nil {
		if existing.WorkspaceID != work.ID || existing.Kind != StoreKind || existing.SourceOnly {
			return core.PersistentResource{}, core.ErrAlreadyExists
		}
		if existing.State != "ready" || !core.ValidPersistentResourceRef(existing.Ref()) {
			return core.PersistentResource{}, fmt.Errorf("automatic Store %s: %w", id, core.ErrRecoveryRequired)
		}
		if err := s.Resources.Backend.Verify(ctx, existing); err != nil {
			return core.PersistentResource{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, core.ErrNotFound) {
		return core.PersistentResource{}, err
	}
	source, err := s.Resources.Store.GetPersistentResource(ctx, PublishedStoreID)
	if errors.Is(err, core.ErrNotFound) {
		return core.PersistentResource{}, nil
	} // no published OCI content
	if err != nil {
		return core.PersistentResource{}, err
	}
	if source.Kind != StoreKind || source.State != "ready" || source.WorkspaceID != "" || !source.SourceOnly {
		return core.PersistentResource{}, core.ErrRecoveryRequired
	}
	return s.Resources.CopyForWorkspace(ctx, id, StoreKind, PublishedStoreID, work.ID)
}
