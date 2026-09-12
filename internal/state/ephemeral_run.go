package state

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *EnvironmentJSONStore) ListEphemeralRuns(_ context.Context) ([]core.EphemeralRun, error) {
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
	runs := make([]core.EphemeralRun, 0, len(data.EphemeralRuns))
	for _, run := range data.EphemeralRuns {
		runs = append(runs, run)
	}
	return runs, nil
}

func (s *EnvironmentJSONStore) PutEphemeralRun(_ context.Context, run core.EphemeralRun) error {
	if err := validateEphemeralRun(run); err != nil {
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
	data.EphemeralRuns[run.EnvironmentID] = run
	// Do not remove or replace evidence supporting an admitted Store lease.
	if err := validatePersistentResourceState(data); err != nil {
		return err
	}
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) DeleteEphemeralRun(_ context.Context, environmentID string) error {
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
	if _, ok := data.EphemeralRuns[environmentID]; !ok {
		return nil
	}
	delete(data.EphemeralRuns, environmentID)
	// Do not remove or replace evidence supporting an admitted Store lease.
	if err := validatePersistentResourceState(data); err != nil {
		return err
	}
	return s.writeEnvironments(data)
}

func validateEphemeralRun(run core.EphemeralRun) error {
	if run.TemporaryWorkspace != nil && !core.ValidTemporaryWorkspace(*run.TemporaryWorkspace) {
		return core.ErrInvalidArgument
	}
	if run.EnvironmentID == "" || run.CreatedAt.IsZero() {
		return core.ErrInvalidArgument
	}
	switch run.State {
	case core.EphemeralRunCreating, core.EphemeralRunActive, core.EphemeralRunCleanupRequired:
		return nil
	default:
		return fmt.Errorf("ephemeral run %q has invalid state %q: %w", run.EnvironmentID, run.State, core.ErrInvalidArgument)
	}
}
