package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const environmentStateVersion = 12 // 9 was an unpublished replacement prototype; reject it.
const previousEnvironmentStateVersion = 2

type environmentFileState struct {
	Restores            map[string]core.SnapshotRestore    `json:"restores,omitempty"`
	BaseAssets          map[string]core.BaseAsset          `json:"base_assets,omitempty"`
	Snapshots           map[string]core.Snapshot           `json:"snapshots,omitempty"`
	PersistentResources map[string]core.PersistentResource `json:"persistent_resources,omitempty"`
	Version             int                                `json:"version"`
	Environments        map[string]core.Environment        `json:"environments"`
	Leases              map[string]core.WorkspaceLease     `json:"workspace_leases,omitempty"`
	EphemeralRuns       map[string]core.EphemeralRun       `json:"ephemeral_runs,omitempty"`
}

type EnvironmentJSONStore struct {
	path string
	mu   sync.Mutex
}

func NewEnvironmentJSONStore(path string) *EnvironmentJSONStore {
	return &EnvironmentJSONStore{path: path}
}

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

func (s *EnvironmentJSONStore) PutEnvironment(_ context.Context, environment core.Environment) error {
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
	data.Environments[environment.Name] = environment
	return s.writeEnvironments(data)
}

func (s *EnvironmentJSONStore) DeleteEnvironment(_ context.Context, name string) error {
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
	if _, ok := data.Environments[name]; !ok {
		return nil
	}
	delete(data.Environments, name)
	return s.writeEnvironments(data)
}

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

func newEnvironmentFileState() environmentFileState {
	return environmentFileState{
		Restores:            map[string]core.SnapshotRestore{},
		BaseAssets:          map[string]core.BaseAsset{},
		Snapshots:           map[string]core.Snapshot{},
		PersistentResources: map[string]core.PersistentResource{},
		Version:             environmentStateVersion,
		Environments:        map[string]core.Environment{},
		Leases:              map[string]core.WorkspaceLease{},
		EphemeralRuns:       map[string]core.EphemeralRun{},
	}
}

func (s *EnvironmentJSONStore) readEnvironments() (environmentFileState, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return newEnvironmentFileState(), nil
	}
	if err != nil {
		return environmentFileState{}, fmt.Errorf("read environment state: %w", err)
	}

	var data environmentFileState
	if err := json.Unmarshal(contents, &data); err != nil {
		return environmentFileState{}, fmt.Errorf("decode environment state: %w", err)
	}
	if data.Environments == nil {
		data.Environments = map[string]core.Environment{}
	}
	if data.Leases == nil {
		data.Leases = map[string]core.WorkspaceLease{}
	}
	if data.EphemeralRuns == nil {
		data.EphemeralRuns = map[string]core.EphemeralRun{}
	}
	if data.Restores == nil {
		data.Restores = map[string]core.SnapshotRestore{}
	}
	if data.BaseAssets == nil {
		data.BaseAssets = map[string]core.BaseAsset{}
	}
	if data.Snapshots == nil {
		data.Snapshots = map[string]core.Snapshot{}
	}
	if data.PersistentResources == nil {
		data.PersistentResources = map[string]core.PersistentResource{}
	}
	if err := normalizeEnvironmentState(&data); err != nil {
		return environmentFileState{}, err
	}
	return data, nil
}

func normalizeEnvironmentState(data *environmentFileState) error {
	if data.Version != 0 && data.Version != 3 && data.Version != 4 && data.Version != 5 && data.Version != 6 && data.Version != 7 && data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != previousEnvironmentStateVersion && data.Version != environmentStateVersion {
		return fmt.Errorf("environment state version %d is unsupported (want %d): %w", data.Version, environmentStateVersion, core.ErrIncompatibleState)
	}

	for id, a := range data.BaseAssets {
		if (data.Version != 7 && data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != environmentStateVersion) || id != a.ID || validateBaseAsset(a) != nil {
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
		if (data.Version != 5 && data.Version != 6 && data.Version != 7 && data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != environmentStateVersion) || id != snapshot.ID || validateSnapshot(snapshot) != nil {
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
		if (data.Version != 8 && data.Version != 10 && data.Version != 11 && data.Version != environmentStateVersion) || id != op.ID || validateRestore(op) != nil || !reflect.DeepEqual(data.Snapshots[op.Saved.ID], op.Saved) || (op.Before.ID != "" && !reflect.DeepEqual(data.Snapshots[op.Before.ID], op.Before)) {
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
			if data.Version != environmentStateVersion || !snapshotIDPattern.MatchString(lease.SnapshotSource) || !ok || saved.State != "ready" || !core.ValidEnvironmentInstanceID(lease.InstanceID) || lease.InstanceID == saved.Source.InstanceID || (lease.State != core.WorkspaceLeaseAcquiring && lease.State != core.WorkspaceLeaseCleanupRequired) {
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

func (s *EnvironmentJSONStore) writeEnvironments(data environmentFileState) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create environment state directory: %w", err)
	}
	data.Version = environmentStateVersion
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode environment state: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".environments-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary environment state: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary environment state: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temporary environment state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary environment state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary environment state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit environment state: %w", err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
