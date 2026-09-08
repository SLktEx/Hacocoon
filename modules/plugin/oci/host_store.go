package oci

import (
	"context"
	"errors"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type HostStoreBackend interface {
	PrepareHostSource(context.Context, core.PersistentResource) error
	VerifyHostSource(context.Context, core.PersistentResource) error
}

// EnsureHost connects the actual Host data area during ordinary setup. It uses
// the canonical source lifecycle; an unfinished setup is never adopted as ready.
func (s WorkspaceStores) EnsureHost(ctx context.Context, backend HostStoreBackend) error {
	if s.Resources == nil || backend == nil {
		return core.ErrInvalidArgument
	}
	source, err := s.Resources.Store.GetPersistentResource(ctx, HostStoreID)
	if err == nil {
		if source.Kind != StoreKind || !source.SourceOnly || source.WorkspaceID != "" || source.State != "ready" {
			return core.ErrRecoveryRequired
		}
		return backend.VerifyHostSource(ctx, source)
	}
	if !errors.Is(err, core.ErrNotFound) {
		return err
	}
	_, err = s.Resources.PublishSource(ctx, HostStoreID, StoreKind, backend.PrepareHostSource)
	return err
}

// RecoverHostCopies is used before ordinary Host setup reads its running state.
// Unconfirmed copies are deliberately left reserved and blocked by the provider.
func (s WorkspaceStores) RecoverHostCopies(ctx context.Context) error {
	if s.Resources == nil {
		return core.ErrInvalidArgument
	}
	resources, err := s.Resources.Store.ListPersistentResources(ctx)
	if err != nil {
		return err
	}
	for _, target := range resources {
		if target.State == "creating" && target.CopyCompleted && target.CopySource.ID == HostStoreID {
			if _, err := s.Resources.RecoverCopy(ctx, target.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
