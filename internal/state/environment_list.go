package state

import (
	"context"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *EnvironmentJSONStore) ListEnvironments(_ context.Context) ([]core.Environment, error) {
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
	environments := make([]core.Environment, 0, len(data.Environments))
	for _, environment := range data.Environments {
		environments = append(environments, environment)
	}
	sort.Slice(environments, func(i, j int) bool {
		return environments[i].Name < environments[j].Name
	})
	return environments, nil
}
