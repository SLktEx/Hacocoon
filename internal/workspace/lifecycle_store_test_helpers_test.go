package workspace

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (f *fakeEnvironmentStore) BeginEnvironmentCreate(ctx context.Context, lease core.WorkspaceLease) error {
	return f.AcquireWorkspaceLease(ctx, lease)
}

func (f *fakeEnvironmentStore) RecordEnvironmentRuntime(_ context.Context, lease core.WorkspaceLease) error {
	existing, ok := f.leases[lease.EnvironmentID]
	if !ok {
		return core.ErrNotFound
	}
	if existing.WorkspaceID != lease.WorkspaceID || existing.SourcePath != lease.SourcePath || existing.AccessMode != lease.AccessMode || existing.Owner != lease.Owner || !existing.AcquiredAt.Equal(lease.AcquiredAt) {
		return core.ErrIncompatibleState
	}
	if lease.State != core.WorkspaceLeaseAcquiring || lease.RuntimeRef == "" {
		return core.ErrInvalidArgument
	}
	if existing.RuntimeRef != "" && existing.RuntimeRef != lease.RuntimeRef {
		return core.ErrIncompatibleState
	}
	f.leases[lease.EnvironmentID] = lease
	return nil
}

func (f *fakeEnvironmentStore) CommitEnvironmentCreate(_ context.Context, environment core.Environment, lease core.WorkspaceLease) error {
	if f.putErr != nil {
		return f.putErr
	}
	existing, ok := f.leases[environment.Name]
	if !ok {
		return core.ErrNotFound
	}
	if existing.State != core.WorkspaceLeaseAcquiring || existing.RuntimeRef == "" || existing.RuntimeRef != environment.RuntimeRef || lease.State != core.WorkspaceLeaseActive || lease.RuntimeRef != environment.RuntimeRef {
		return fmt.Errorf("invalid ready transition: %w", core.ErrIncompatibleState)
	}
	f.environments[environment.Name] = environment
	f.leases[environment.Name] = lease
	return nil
}

func (f *fakeEnvironmentStore) MarkEnvironmentRecoveryRequired(_ context.Context, lease core.WorkspaceLease) error {
	existing, ok := f.leases[lease.EnvironmentID]
	if !ok {
		return core.ErrNotFound
	}
	if lease.State != core.WorkspaceLeaseCleanupRequired {
		return core.ErrInvalidArgument
	}
	if existing.RuntimeRef != "" && lease.RuntimeRef == "" {
		lease.RuntimeRef = existing.RuntimeRef
	}
	if existing.RuntimeRef != "" && lease.RuntimeRef != existing.RuntimeRef {
		return core.ErrIncompatibleState
	}
	f.leases[lease.EnvironmentID] = lease
	return nil
}

func (f *fakeEnvironmentStore) FinalizeEnvironmentDelete(_ context.Context, environmentID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.environments[environmentID]; ok {
		f.deleted = append(f.deleted, environmentID)
	}
	delete(f.environments, environmentID)
	delete(f.leases, environmentID)
	return nil
}
