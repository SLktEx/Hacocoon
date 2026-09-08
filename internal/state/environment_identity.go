package state

import (
	"context"
	"encoding/json"
	"os"
	"reflect"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// EnvironmentInstance returns a durable creation identity only for this exact
// ready Environment snapshot. The state lock keeps its lease and metadata
// observation atomic. The identifier is not a provider authority credential.
func (s *EnvironmentJSONStore) EnvironmentInstance(ctx context.Context, expected core.Environment) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return "", err
	}
	defer unlock()
	// Read raw evidence: the general legacy reader may synthesize a missing
	// lease, which is insufficient proof for a durable approval identity.
	content, err := os.ReadFile(s.path)
	if err != nil {
		return "", err
	}
	var data environmentFileState
	if err = json.Unmarshal(content, &data); err != nil {
		return "", err
	}
	if data.Version != 0 && data.Version != 3 && data.Version != 4 && data.Version != 5 && data.Version != 6 && data.Version != 7 && data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != 12 && data.Version != previousEnvironmentStateVersion && data.Version != environmentStateVersion {
		return "", core.ErrIncompatibleState
	}
	current, ok := data.Environments[expected.Name]
	if !ok {
		return "", core.ErrNotFound
	}
	if !reflect.DeepEqual(current, expected) {
		return "", core.ErrCapabilityStale
	}
	lease, ok := data.Leases[expected.Name]
	if !ok || lease.SnapshotSource != "" || lease.Owner == "" || lease.AcquiredAt.IsZero() {
		return "", core.ErrIncompatibleState
	}
	if err := validateEnvironmentCreateCommit(current, lease); err != nil {
		return "", err
	}
	if lease.InstanceID == "" {
		// Legacy migration only: preserve the existing resource and assign its
		// missing identity once, under the same catalog lock. Never derive it from
		// a reusable name, provider reference, owner label or wall-clock timestamp.
		lease.InstanceID, err = core.NewEnvironmentInstanceID()
		if err != nil {
			return "", err
		}
		if err = ctx.Err(); err != nil {
			return "", err
		}
		data.Leases[expected.Name] = lease
		if err = s.writeEnvironments(data); err != nil {
			return "", err
		}
	}
	if !core.ValidEnvironmentInstanceID(lease.InstanceID) {
		return "", core.ErrIncompatibleState
	}
	return lease.InstanceID, nil
}
