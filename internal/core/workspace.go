package core

import "time"

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
	WorkspaceID       WorkspaceID         `json:"workspace_id"`
	SourcePath        string              `json:"source_path"`
	EnvironmentID     string              `json:"environment_id"`
	AccessMode        WorkspaceAccessMode `json:"access_mode"`
	Owner             string              `json:"owner"`
	CreateOperationID string              `json:"create_operation_id,omitempty"`
	RuntimeRef        string              `json:"runtime_ref,omitempty"`
	State             WorkspaceLeaseState `json:"state,omitempty"`
	AcquiredAt        time.Time           `json:"acquired_at"`
}

// EphemeralRun is trusted host-side evidence that an Environment belongs to
// haco run. Names alone are never sufficient proof because a user may create an
// ordinary Environment whose name happens to start with "run-".
type EphemeralRun struct {
	EnvironmentID string            `json:"environment_id"`
	State         EphemeralRunState `json:"state"`
	CreatedAt     time.Time         `json:"created_at"`
}

type Environment struct {
	Name       string              `json:"name"`
	Workspace  Workspace           `json:"workspace"`
	AccessMode WorkspaceAccessMode `json:"access_mode"`
	Base       *BaseRef            `json:"base,omitempty"`
	Resources  ResourceBudget      `json:"resources"`
	RuntimeRef string              `json:"runtime_ref"`
	CreatedAt  time.Time           `json:"created_at"`
}

type EnvironmentSpec struct {
	Name          string
	WorkspacePath string
	AccessMode    WorkspaceAccessMode
	Base          BaseName
	Resources     ResourceBudget
}

type EnvironmentRuntimeSpec struct {
	Name              string
	WorkspacePath     string
	ReadOnly          bool
	Base              BaseName
	Resources         ResourceBudget
	CreateOperationID string
}

type EnvironmentRuntime struct {
	Ref       string
	Base      *BaseRef
	Resources ResourceBudget
}

type ExecutionRequest struct {
	Argv []string
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
