package core

import (
	"reflect"
	"slices"
	"time"
)

type WorkspaceID string

type WorkspaceAccessMode string

type WorkspaceLeaseState string

type EphemeralRunState string

type BaseName string

type BaseRevision string

const (
	WorkspaceReadOnly  WorkspaceAccessMode = "ro"
	WorkspaceReadWrite WorkspaceAccessMode = "rw"

	WorkspaceLeaseAcquiring       WorkspaceLeaseState = "acquiring"
	WorkspaceLeaseActive          WorkspaceLeaseState = "active"
	WorkspaceLeaseCleanupRequired WorkspaceLeaseState = "cleanup-required"

	EphemeralRunCreating        EphemeralRunState = "creating"
	EphemeralRunActive          EphemeralRunState = "active"
	EphemeralRunCleanupRequired EphemeralRunState = "cleanup-required"
)

type Workspace struct {
	ID   WorkspaceID `json:"id"`
	Path string      `json:"path"`
}

type BaseRef struct {
	Name     BaseName     `json:"name"`
	Revision BaseRevision `json:"revision"`
}

type BaseInfo struct {
	Name     BaseName     `json:"name"`
	Revision BaseRevision `json:"revision,omitempty"`
}

type WorkspaceLease struct {
	Attachments   []EnvironmentAttachment `json:"attachments,omitempty"`
	RuntimeAbsent bool                    `json:"runtime_absent,omitempty"`
	Ephemeral     bool                    `json:"ephemeral,omitempty"`
	// SnapshotSource reserves immutable saved data only while this creation is pending.
	SnapshotSource     string                `json:"snapshot_source,omitempty"`
	InstanceID         string                `json:"instance_id,omitempty"`
	PersistentResource PersistentResourceRef `json:"persistent_resource,omitempty"`
	WorkspaceID        WorkspaceID           `json:"workspace_id"`
	SourcePath         string                `json:"source_path"`
	EnvironmentID      string                `json:"environment_id"`
	AccessMode         WorkspaceAccessMode   `json:"access_mode"`
	Owner              string                `json:"owner"`
	RuntimeRef         string                `json:"runtime_ref,omitempty"`
	State              WorkspaceLeaseState   `json:"state,omitempty"`
	AcquiredAt         time.Time             `json:"acquired_at"`
}

// EphemeralRun is trusted host-side evidence that an Environment belongs to
// haco run. Names alone are never sufficient proof because a user may create an
// ordinary Environment whose name happens to start with "run-".
type EphemeralRun struct {
	InstanceID         string            `json:"instance_id,omitempty"`
	TemporaryWorkspace *Workspace        `json:"temporary_workspace,omitempty"`
	EnvironmentID      string            `json:"environment_id"`
	State              EphemeralRunState `json:"state"`
	CreatedAt          time.Time         `json:"created_at"`
}

type Environment struct {
	DNSMode            DNSMode                 `json:"dns_mode,omitempty"`
	Attachments        []EnvironmentAttachment `json:"attachments,omitempty"`
	PersistentResource PersistentResourceRef   `json:"persistent_resource,omitempty"`
	Name               string                  `json:"name"`
	Workspace          Workspace               `json:"workspace"`
	AccessMode         WorkspaceAccessMode     `json:"access_mode"`
	Base               *BaseRef                `json:"base,omitempty"`
	Resources          ResourceBudget          `json:"resources"`
	RuntimeRef         string                  `json:"runtime_ref"`
	CreatedAt          time.Time               `json:"created_at"`
}

type EnvironmentSpec struct {
	DNSMode DNSMode
	// EphemeralInstance binds a trusted run reservation to canonical creation.
	// It is not accepted by ordinary client Environment-create DTOs.
	EphemeralInstance string
	// ExpectedWorkspace pins a reviewed retained work before any provider mutation.
	ExpectedWorkspace   WorkspaceID
	TemporaryWorkspace  *Workspace
	SkipDefaultResource bool
	PersistentResource  string
	// ExpectedResource pins an explicit resource to its reviewed owner.
	ExpectedResource PersistentResourceRef
	Name             string
	WorkspacePath    string
	AccessMode       WorkspaceAccessMode
	Base             BaseName
	Resources        ResourceBudget
}

type EnvironmentRuntimeSpec struct {
	DNSMode     DNSMode
	Attachments []EnvironmentRuntimeAttachment
	// InstanceID binds the provider resource to the durable creation reservation.
	InstanceID         string
	TemporaryWorkspace bool
	// ResourceMaintenance requires preparation before retained data attachment.
	// Runtimes must refuse it until that sequence is supported.
	ResourceMaintenance bool
	PersistentResource  PersistentResource
	Name                string
	WorkspacePath       string
	ReadOnly            bool
	Base                BaseName
	Resources           ResourceBudget
}

type EnvironmentRuntime struct {
	Ref       string
	Base      *BaseRef
	Resources ResourceBudget
}

const MaxExecutionInputBytes = 1 << 20

type ExecutionRequest struct {
	Stdin            []byte
	WorkingDirectory string
	Argv             []string
}

type ExecutionResult struct {
	ExitCode        int
	Stdout          string
	Stderr          string
	StdoutTruncated bool
	StderrTruncated bool
	StdoutBytes     int64
	StderrBytes     int64
}

// MatchesEnvironment checks the common active-lease binding. Generation,
// snapshot and authority-specific checks remain with their operation owners.
func (lease WorkspaceLease) MatchesEnvironment(environment Environment) bool {
	return lease.State == WorkspaceLeaseActive && !lease.RuntimeAbsent && environment.Name != "" &&
		slices.Equal(lease.Attachments, environment.Attachments) &&
		lease.EnvironmentID == environment.Name && lease.RuntimeRef != "" &&
		lease.RuntimeRef == environment.RuntimeRef &&
		lease.WorkspaceID == environment.Workspace.ID &&
		lease.SourcePath == environment.Workspace.Path &&
		lease.AccessMode == environment.AccessMode &&
		lease.PersistentResource == environment.PersistentResource
}

// Equal includes every aggregate field, including independently decoded Base and attachment values.
func (e Environment) Equal(other Environment) bool {
	same := slices.Equal(e.Attachments, other.Attachments)
	e.Attachments, other.Attachments = nil, nil
	return same && reflect.DeepEqual(e, other)
}

func (lease WorkspaceLease) Equal(other WorkspaceLease) bool {
	same := slices.Equal(lease.Attachments, other.Attachments)
	lease.Attachments, other.Attachments = nil, nil
	return same && reflect.DeepEqual(lease, other)
}
