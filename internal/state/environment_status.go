package state

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// GetReadyEnvironment observes metadata and the reservation in one catalog
// transaction. Unlike lifecycle lookup, status must report incomplete cleanup;
// it never repairs ownership, assigns identities or writes legacy migration.
func (s *EnvironmentJSONStore) GetReadyEnvironment(ctx context.Context, name string) (core.Environment, error) {
	var environment core.Environment
	err := s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		current, exists := data.Environments[name]
		lease, reserved := data.Leases[name]
		if !exists {
			if reserved {
				return false, fmt.Errorf("environment %q has an incomplete lifecycle: %w", name, core.ErrRecoveryRequired)
			}
			return false, fmt.Errorf("environment %q: %w", name, core.ErrNotFound)
		}
		if !reserved || current.Name != name || !lease.MatchesEnvironment(current) {
			return false, fmt.Errorf("environment %q ownership is not ready: %w", name, core.ErrRecoveryRequired)
		}
		environment = current
		return false, nil
	})
	return environment, err
}
