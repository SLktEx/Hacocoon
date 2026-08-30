package state

import (
	"context"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ListEnvironments returns the trusted persisted Environment set in a
// deterministic order. It intentionally shares the same process/file lock and
// state-version validation used by the point lookup methods.
func (s *EnvironmentJSONStore) ListEnvironments(_ context.Context) ([]core.Environment, error) {
	if s == nil {
		return nil, core.ErrInvalidArgument
	}
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
	result := make([]core.Environment, 0, len(data.Environments))
	for _, environment := range data.Environments {
		result = append(result, environment)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
