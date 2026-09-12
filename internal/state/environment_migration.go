package state

import (
	"fmt"
	"reflect"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func normalizeEnvironmentState(data *environmentFileState) error {
	if data.Version != 0 && data.Version != 3 && data.Version != 4 && data.Version != 5 && data.Version != 6 && data.Version != 7 && data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != 12 && data.Version != previousEnvironmentStateVersion && data.Version != environmentStateVersion {
		return fmt.Errorf("environment state version %d is unsupported (want %d): %w", data.Version, environmentStateVersion, core.ErrIncompatibleState)
	}

	for id, copy := range data.WorkspaceCopies {
		saved, ok := data.Snapshots[copy.SnapshotID]
		if data.Version != environmentStateVersion || !validSnapshotWorkspaceCopy(id, copy) || !ok || saved.State != "ready" {
			return core.ErrIncompatibleState
		}
	}
	for id, a := range data.BaseAssets {
		if (data.Version != 7 && data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != 12 && data.Version != environmentStateVersion) || id != a.ID || validateBaseAsset(a) != nil {
			return core.ErrIncompatibleState
		}
		for otherID, b := range data.BaseAssets {
			if otherID != id && a.Provider == b.Provider && ((a.Scope == b.Scope && a.Base == b.Base) || a.NativeRef == b.NativeRef) {
				return core.ErrIncompatibleState
			}
		}
	}
	for id, snapshot := range data.Snapshots {
		if data.Version == 5 {
			for _, component := range snapshot.Components {
				if component.Binding != "" || component.Role == "base" {
					return core.ErrIncompatibleState
				}
			}
		}
		if (data.Version != 5 && data.Version != 6 && data.Version != 7 && data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != 12 && data.Version != environmentStateVersion) || id != snapshot.ID || validateSnapshot(snapshot) != nil {
			return core.ErrIncompatibleState
		}
	}
	if data.Version == 8 {
		for id, op := range data.Restores {
			if op.Current.Environment.Name != "" || op.Before.ID == "" {
				return core.ErrIncompatibleState
			}
			op.Current = op.Before.Source
			data.Restores[id] = op
		}
	}
	for id, op := range data.Restores {
		if (data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != 12 && data.Version != environmentStateVersion) || id != op.ID || validateRestore(op) != nil || !reflect.DeepEqual(data.Snapshots[op.Saved.ID], op.Saved) || (op.Before.ID != "" && !reflect.DeepEqual(data.Snapshots[op.Before.ID], op.Before)) {
			return core.ErrIncompatibleState
		}
		before := op.Current
		lease := data.Leases[before.Environment.Name]
		if !reflect.DeepEqual(data.Environments[before.Environment.Name], before.Environment) || lease.InstanceID != before.InstanceID || lease.State != core.WorkspaceLeaseActive || validateEnvironmentCreateCommit(before.Environment, lease) != nil {
			return core.ErrIncompatibleState
		}
		for _, c := range op.Components {
			for _, env := range data.Environments {
				if c.NativeRef == env.RuntimeRef {
					return core.ErrIncompatibleState
				}
			}
			for _, snap := range data.Snapshots {
				for _, old := range snap.Components {
					if c.NativeRef == old.NativeRef || c.Owner == old.Owner {
						return core.ErrIncompatibleState
					}
				}
			}
			for _, a := range data.BaseAssets {
				if c.NativeRef == a.NativeRef || c.Owner == a.Owner {
					return core.ErrIncompatibleState
				}
			}
		}
		for otherID, other := range data.Restores {
			if otherID == id {
				continue
			}
			if op.Current.Environment.Name == other.Current.Environment.Name {
				return core.ErrIncompatibleState
			}
			for _, a := range op.Components {
				for _, b := range other.Components {
					if a.NativeRef == b.NativeRef || a.Owner == b.Owner {
						return core.ErrIncompatibleState
					}
				}
			}
		}
	}
	for name, environment := range data.Environments {
		if environment.Name == "" {
			environment.Name = name
		}
		data.Environments[name] = environment
		if !environmentSupportsLease(environment) {
			continue
		}
		if _, ok := data.Leases[name]; !ok {
			data.Leases[name] = core.WorkspaceLease{
				PersistentResource: environment.PersistentResource,
				WorkspaceID:        environment.Workspace.ID,
				SourcePath:         environment.Workspace.Path,
				EnvironmentID:      name,
				AccessMode:         environment.AccessMode,
				Owner:              name,
				RuntimeRef:         environment.RuntimeRef,
				State:              core.WorkspaceLeaseActive,
				AcquiredAt:         environment.CreatedAt,
			}
		}
	}

	for environmentID, lease := range data.Leases {
		if lease.SnapshotSource != "" {
			saved, ok := data.Snapshots[lease.SnapshotSource]
			if (data.Version != 12 && data.Version != environmentStateVersion) || !snapshotIDPattern.MatchString(lease.SnapshotSource) || !ok || saved.State != "ready" || !core.ValidEnvironmentInstanceID(lease.InstanceID) || lease.InstanceID == saved.Source.InstanceID || (lease.State != core.WorkspaceLeaseAcquiring && lease.State != core.WorkspaceLeaseCleanupRequired) {
				return core.ErrIncompatibleState
			}
		}
		if lease.EnvironmentID == "" {
			lease.EnvironmentID = environmentID
		}
		if lease.State == "" {
			lease.State = core.WorkspaceLeaseActive
		}
		if lease.RuntimeRef == "" {
			if environment, ok := data.Environments[environmentID]; ok {
				lease.RuntimeRef = environment.RuntimeRef
			}
		}
		data.Leases[environmentID] = lease
	}

	for environmentID, run := range data.EphemeralRuns {
		if run.EnvironmentID == "" {
			run.EnvironmentID = environmentID
		}
		if run.State == "" {
			run.State = core.EphemeralRunCleanupRequired
		}
		data.EphemeralRuns[environmentID] = run
	}
	if err := validatePersistentResourceState(*data); err != nil {
		return err
	}
	data.Version = environmentStateVersion
	return nil
}

func validateLeaseCompatibleState(data environmentFileState) error {
	for name, environment := range data.Environments {
		if !environmentSupportsLease(environment) {
			return fmt.Errorf("environment %q uses pre-v0.2 metadata; delete v0.1 environments before creating leased workspaces: %w", name, core.ErrIncompatibleState)
		}
	}
	return nil
}

func environmentSupportsLease(environment core.Environment) bool {
	return environment.Workspace.ID != "" &&
		environment.Workspace.Path != "" &&
		environment.AccessMode != "" &&
		environment.RuntimeRef != ""
}
