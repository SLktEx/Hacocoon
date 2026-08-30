package core

import "time"

type WorkspaceID string

type WorkspaceAccessMode string

type WorkspaceLeaseState string

type BaseName string

type BaseRevision string

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

type BaseRef struct {
	Name     BaseName     `json:"name"`
	Revision BaseRevision `json:"revision"`
}

type BaseInfo struct {
	Name     BaseName     `json:"name"`
	Revision BaseRevision `json:"revision,omitempty"`
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
	Base       *BaseRef            `json:"base,omitempty"`
	RuntimeRef string              `json:"runtime_ref"`
	CreatedAt  time.Time           `json:"created_at"`
}

type EnvironmentSpec struct {
	Name          string
	WorkspacePath string
	AccessMode    WorkspaceAccessMode
	Base          BaseName
}

type EnvironmentRuntimeSpec struct {
	Name          string
	WorkspacePath string
	ReadOnly      bool
	Base          BaseName
}

type EnvironmentRuntime struct {
	Ref  string
	Base *BaseRef
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
