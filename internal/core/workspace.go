package core

import "time"

type WorkspaceID string

type WorkspaceAccessMode string

type WorkspaceLeaseState string

const (
	WorkspaceReadOnly  WorkspaceAccessMode = "ro"
	WorkspaceReadWrite WorkspaceAccessMode = "rw"

	WorkspaceLeaseAcquiring       WorkspaceLeaseState = "acquiring"
	WorkspaceLeaseActive          WorkspaceLeaseState = "active"
	WorkspaceLeaseCleanupRequired WorkspaceLeaseState = "cleanup-required"
)

type Workspace struct {
	ID   WorkspaceID `json:"id"`
	Path string      `json:"path"`
}

type WorkspaceLease struct {
	WorkspaceID   WorkspaceID         `json:"workspace_id"`
	SourcePath    string              `json:"source_path"`
	EnvironmentID string              `json:"environment_id"`
	AccessMode    WorkspaceAccessMode `json:"access_mode"`
	Owner         string              `json:"owner"`
	RuntimeRef    string              `json:"runtime_ref,omitempty"`
	State         WorkspaceLeaseState `json:"state,omitempty"`
	AcquiredAt    time.Time           `json:"acquired_at"`
}

type Environment struct {
	Name       string              `json:"name"`
	Workspace  Workspace           `json:"workspace"`
	AccessMode WorkspaceAccessMode `json:"access_mode"`
	RuntimeRef string              `json:"runtime_ref"`
	CreatedAt  time.Time           `json:"created_at"`
}

type EnvironmentSpec struct {
	Name          string
	WorkspacePath string
	AccessMode    WorkspaceAccessMode
}

type EnvironmentRuntimeSpec struct {
	Name          string
	WorkspacePath string
	ReadOnly      bool
}

type EnvironmentRuntime struct {
	Ref string
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
}
