package state

import "github.com/SLktEx/Hacocoon/internal/core"

// Missing identities in legacy records remain explicit recovery work. Never
// derive one from a name or adopt a current Environment on read.
func validateEphemeralIdentities(data environmentFileState) error {
	seen := map[string]bool{}
	for name, run := range data.EphemeralRuns {
		if name != run.EnvironmentID || validateEphemeralRun(run) != nil {
			return core.ErrIncompatibleState
		}
		if run.InstanceID == "" {
			continue
		}
		if seen[run.InstanceID] {
			return core.ErrIncompatibleState
		}
		seen[run.InstanceID] = true
		if lease, ok := data.Leases[name]; ok && (!lease.Ephemeral || lease.InstanceID != run.InstanceID) {
			return core.ErrIncompatibleState
		}
		if lease, ok := data.Leases[name]; ok && run.TemporaryWorkspace != nil && (lease.WorkspaceID != run.TemporaryWorkspace.ID || lease.SourcePath != run.TemporaryWorkspace.Path) {
			return core.ErrIncompatibleState
		}
	}
	for name, lease := range data.Leases {
		if lease.Ephemeral {
			run, ok := data.EphemeralRuns[name]
			if !ok || !core.ValidEnvironmentInstanceID(lease.InstanceID) || run.InstanceID != lease.InstanceID {
				return core.ErrIncompatibleState
			}
		}
	}
	return nil
}
