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
