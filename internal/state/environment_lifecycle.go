package state

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// BeginEnvironmentCreate atomically reserves an Environment identity and its
// Workspace lease before any provider-side resource is created.
//
// The reservation deliberately remains in the acquiring state until the
// provider runtime reference has been durably recorded and the complete
// Environment can be committed. Callers should use the lifecycle methods in
// this file rather than composing PutEnvironment/PutWorkspaceLease manually.
func (s *EnvironmentJSONStore) BeginEnvironmentCreate(_ context.Context, lease core.WorkspaceLease) error {
	if err := validateEnvironmentCreateReservation(lease); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if err := validateLeaseCompatibleState(data); err != nil {
		return err
	}
	if _, ok := data.Environments[lease.EnvironmentID]; ok {
		return fmt.Errorf("environment %q: %w", lease.EnvironmentID, core.ErrAlreadyExists)
	}
	if _, ok := data.Leases[lease.EnvironmentID]; ok {
		return fmt.Errorf("workspace lease for environment %q already exists: %w", lease.EnvironmentID, core.ErrAlreadyExists)
	}
	for _, existing := range data.Leases {
		if existing.WorkspaceID != lease.WorkspaceID {
			continue
		}
		if existing.AccessMode == core.WorkspaceReadWrite || lease.AccessMode == core.WorkspaceReadWrite {
			return fmt.Errorf("workspace %s already leased by environment %q (%s): %w", lease.WorkspaceID, existing.EnvironmentID, existing.AccessMode, core.ErrWorkspaceBusy)
		}
	}

	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

// RecordEnvironmentRuntime durably records provider ownership immediately
// after provider creation succeeds, while keeping the lease in acquiring state.
// This closes the dangerous window where a runtime exists but no durable
// Hacocoon record can identify it if the next persistence operation fails.
func (s *EnvironmentJSONStore) RecordEnvironmentRuntime(_ context.Context, lease core.WorkspaceLease) error {
	if err := validateEnvironmentRuntimeReservation(lease); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if _, ok := data.Environments[lease.EnvironmentID]; ok {
		return fmt.Errorf("environment %q was published before runtime reservation commit: %w", lease.EnvironmentID, core.ErrIncompatibleState)
	}
	existing, ok := data.Leases[lease.EnvironmentID]
	if !ok {
		return fmt.Errorf("workspace lease for environment %q: %w", lease.EnvironmentID, core.ErrNotFound)
	}
	if err := validateSameLeaseReservation(existing, lease); err != nil {
		return err
	}
	if existing.RuntimeRef != "" && existing.RuntimeRef != lease.RuntimeRef {
		return fmt.Errorf("workspace lease for environment %q already owns runtime %q, refusing replacement with %q: %w", lease.EnvironmentID, existing.RuntimeRef, lease.RuntimeRef, core.ErrIncompatibleState)
	}

	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

// CommitEnvironmentCreate atomically publishes the Environment and activates
// its Workspace lease. There is no durable state in which a lease is active
// while the corresponding ready Environment metadata is still absent.
func (s *EnvironmentJSONStore) CommitEnvironmentCreate(_ context.Context, environment core.Environment, lease core.WorkspaceLease) error {
	if err := validateEnvironmentCreateCommit(environment, lease); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	if existingEnvironment, ok := data.Environments[environment.Name]; ok {
		existingLease, leaseOK := data.Leases[environment.Name]
		if leaseOK && existingEnvironment == environment && existingLease == lease {
			return nil
		}
		return fmt.Errorf("environment %q already has different committed state: %w", environment.Name, core.ErrIncompatibleState)
	}

	existingLease, ok := data.Leases[environment.Name]
	if !ok {
		return fmt.Errorf("workspace lease for environment %q: %w", environment.Name, core.ErrNotFound)
	}
	if err := validateSameLeaseReservation(existingLease, lease); err != nil {
		return err
	}
	if existingLease.State != core.WorkspaceLeaseAcquiring {
		return fmt.Errorf("workspace lease for environment %q is %q, want %q before ready commit: %w", environment.Name, existingLease.State, core.WorkspaceLeaseAcquiring, core.ErrIncompatibleState)
	}
	if existingLease.RuntimeRef == "" || existingLease.RuntimeRef != environment.RuntimeRef || existingLease.RuntimeRef != lease.RuntimeRef {
		return fmt.Errorf("environment %q runtime ownership was not durably recorded before ready commit: %w", environment.Name, core.ErrIncompatibleState)
	}

	data.Environments[environment.Name] = environment
	data.Leases[environment.Name] = lease
	return s.writeEnvironments(data)
}

// MarkEnvironmentRecoveryRequired preserves conservative ownership when a
// provider resource may still exist after a failed create/cleanup operation.
// It never clears a previously recorded runtime reference.
func (s *EnvironmentJSONStore) MarkEnvironmentRecoveryRequired(_ context.Context, lease core.WorkspaceLease) error {
	if lease.EnvironmentID == "" || lease.State != core.WorkspaceLeaseCleanupRequired {
		return core.ErrInvalidArgument
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	existing, ok := data.Leases[lease.EnvironmentID]
	if !ok {
		return fmt.Errorf("workspace lease for environment %q: %w", lease.EnvironmentID, core.ErrNotFound)
	}
	if err := validateSameLeaseReservation(existing, lease); err != nil {
		return err
	}
	if existing.RuntimeRef != "" && lease.RuntimeRef == "" {
		lease.RuntimeRef = existing.RuntimeRef
	}
	if existing.RuntimeRef != "" && lease.RuntimeRef != existing.RuntimeRef {
		return fmt.Errorf("workspace lease for environment %q owns runtime %q, refusing recovery rewrite to %q: %w", lease.EnvironmentID, existing.RuntimeRef, lease.RuntimeRef, core.ErrIncompatibleState)
	}

	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

// FinalizeEnvironmentDelete atomically forgets ready Environment metadata and
// its Workspace lease only after the caller has positively established that the
// provider runtime is absent. Retrying after a persistence failure therefore
// cannot expose a Workspace as free while stale Environment metadata remains,
// or vice versa.
func (s *EnvironmentJSONStore) FinalizeEnvironmentDelete(_ context.Context, environmentID string) error {
	if environmentID == "" {
		return core.ErrInvalidArgument
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	_, environmentExists := data.Environments[environmentID]
	_, leaseExists := data.Leases[environmentID]
	if !environmentExists && !leaseExists {
		return nil
	}
	delete(data.Environments, environmentID)
	delete(data.Leases, environmentID)
	return s.writeEnvironments(data)
}

func validateEnvironmentCreateReservation(lease core.WorkspaceLease) error {
	if lease.EnvironmentID == "" || lease.WorkspaceID == "" || lease.SourcePath == "" || lease.Owner == "" || lease.RuntimeRef != "" || lease.State != core.WorkspaceLeaseAcquiring || lease.AcquiredAt.IsZero() {
		return core.ErrInvalidArgument
	}
	switch lease.AccessMode {
	case core.WorkspaceReadOnly, core.WorkspaceReadWrite:
		return nil
	default:
		return core.ErrInvalidArgument
	}
}

func validateEnvironmentRuntimeReservation(lease core.WorkspaceLease) error {
	if err := validateEnvironmentCreateReservation(core.WorkspaceLease{
		WorkspaceID:   lease.WorkspaceID,
		SourcePath:    lease.SourcePath,
		EnvironmentID: lease.EnvironmentID,
		AccessMode:    lease.AccessMode,
		Owner:         lease.Owner,
		State:         lease.State,
		AcquiredAt:    lease.AcquiredAt,
	}); err != nil {
		return err
	}
	if lease.RuntimeRef == "" {
		return core.ErrInvalidArgument
	}
	return nil
}

func validateEnvironmentCreateCommit(environment core.Environment, lease core.WorkspaceLease) error {
	if environment.Name == "" || environment.RuntimeRef == "" || environment.Workspace.ID == "" || environment.Workspace.Path == "" || environment.CreatedAt.IsZero() {
		return core.ErrInvalidArgument
	}
	if lease.EnvironmentID != environment.Name || lease.RuntimeRef != environment.RuntimeRef || lease.WorkspaceID != environment.Workspace.ID || lease.SourcePath != environment.Workspace.Path || lease.AccessMode != environment.AccessMode || lease.State != core.WorkspaceLeaseActive {
		return fmt.Errorf("environment %q and Workspace lease do not describe the same ready resource aggregate: %w", environment.Name, core.ErrIncompatibleState)
	}
	return nil
}

func validateSameLeaseReservation(existing, next core.WorkspaceLease) error {
	if existing.EnvironmentID != next.EnvironmentID || existing.WorkspaceID != next.WorkspaceID || existing.SourcePath != next.SourcePath || existing.AccessMode != next.AccessMode || existing.Owner != next.Owner || !existing.AcquiredAt.Equal(next.AcquiredAt) {
		return fmt.Errorf("workspace lease reservation for environment %q changed identity during lifecycle transition: %w", next.EnvironmentID, core.ErrIncompatibleState)
	}
	return nil
}
