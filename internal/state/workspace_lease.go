package state

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *EnvironmentJSONStore) ListWorkspaceLeases(_ context.Context) ([]core.WorkspaceLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return nil, err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return nil, err
	}
	leases := make([]core.WorkspaceLease, 0, len(data.Leases))
	for _, lease := range data.Leases {
		leases = append(leases, lease)
	}
	return leases, nil
}

func (s *EnvironmentJSONStore) GetWorkspaceLease(_ context.Context, environmentID string) (core.WorkspaceLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return core.WorkspaceLease{}, err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return core.WorkspaceLease{}, err
	}
	lease, ok := data.Leases[environmentID]
	if !ok {
		return core.WorkspaceLease{}, fmt.Errorf("workspace lease for environment %q: %w", environmentID, core.ErrNotFound)
	}
	return lease, nil
}

func (s *EnvironmentJSONStore) AcquireWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
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
	if lease.State == "" {
		lease.State = core.WorkspaceLeaseAcquiring
	}
	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) PutWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
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
	if _, ok := data.Leases[lease.EnvironmentID]; !ok {
		return fmt.Errorf("workspace lease for environment %q: %w", lease.EnvironmentID, core.ErrNotFound)
	}
	data.Leases[lease.EnvironmentID] = lease
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) DeleteWorkspaceLease(_ context.Context, environmentID string) error {
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
	if _, ok := data.Leases[environmentID]; !ok {
		return nil
	}
	delete(data.Leases, environmentID)
	return s.writeEnvironments(data)
}
