package oci

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
)

// HostStoreID is outside the user-managed oci: namespace. Only trusted Host
// storage setup may own this source; guest Stores never return to Host.
const HostStoreID = "oci-source:host"

type WorkspaceStores struct{ Resources *persistentresource.Service }

// Resolve runs under the canonical Workspace lock. Existing guest changes win
// over newer publications; a failed copy remains visible and recovery-required.
func (s WorkspaceStores) Resolve(ctx context.Context, work core.Workspace) (core.PersistentResource, error) {
	if s.Resources == nil || work.ID == "" {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	id := workspaceStoreID(work.ID)
	existing, err := s.Resources.Store.GetPersistentResource(ctx, id)
	if err == nil {
		if existing.WorkspaceID != work.ID || existing.Kind != StoreKind || existing.SourceOnly {
			return core.PersistentResource{}, core.ErrAlreadyExists
		}
		if existing.State == "creating" && existing.CopyCompleted {
			return s.Resources.RecoverCopy(ctx, id)
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
	source, err := s.Resources.Store.GetPersistentResource(ctx, HostStoreID)
	if errors.Is(err, core.ErrNotFound) {
		return core.PersistentResource{}, nil
	} // no published OCI content
	if err != nil {
		return core.PersistentResource{}, err
	}
	if source.Kind != StoreKind || source.State != "ready" || source.WorkspaceID != "" || !source.SourceOnly {
		return core.PersistentResource{}, core.ErrRecoveryRequired
	}
	return s.Resources.CopyForWorkspace(ctx, id, StoreKind, HostStoreID, work.ID)
}

func workspaceStoreID(work core.WorkspaceID) string {
	sum := sha256.Sum256([]byte(work))
	return fmt.Sprintf("oci:auto-%x", sum[:16])
}

// CleanupTemporary only disposes the default copy bound to this exact scratch
// Workspace after canonical Environment removal. Incomplete copies fail closed.
func (s WorkspaceStores) CleanupTemporary(ctx context.Context, work core.Workspace) error {
	if s.Resources == nil || !core.ValidTemporaryWorkspace(work) {
		return core.ErrInvalidArgument
	}
	err := s.Resources.DeleteForWorkspace(ctx, workspaceStoreID(work.ID), work.ID)
	if errors.Is(err, core.ErrNotFound) {
		return nil
	}
	return err
}
