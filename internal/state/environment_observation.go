package state

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *EnvironmentJSONStore) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return core.Environment{}, err
	}
	defer unlock()

	data, err := s.readEnvironments()
	if err != nil {
		return core.Environment{}, err
	}
	environment, ok := data.Environments[name]
	if !ok {
		return core.Environment{}, fmt.Errorf("environment %q: %w", name, core.ErrNotFound)
	}
	return environment, nil
}
