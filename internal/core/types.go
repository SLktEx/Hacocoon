package core

import "time"

type SessionID string

type DesiredState string

type ObservedState string

const (
	DesiredRunning DesiredState = "running"
	DesiredStopped DesiredState = "stopped"
	DesiredDeleted DesiredState = "deleted"

	ObservedUnknown ObservedState = "unknown"
	ObservedRunning ObservedState = "running"
	ObservedStopped ObservedState = "stopped"
	ObservedError   ObservedState = "error"
)

type SessionSpec struct {
	Name string
}

type Session struct {
	ID            SessionID     `json:"id"`
	Name          string        `json:"name"`
	RuntimeModule string        `json:"runtime_module"`
	RuntimeRef    string        `json:"runtime_ref"`
	StorageModule string        `json:"storage_module"`
	StorageRef    string        `json:"storage_ref"`
	DesiredState  DesiredState  `json:"desired_state"`
	ObservedState ObservedState `json:"observed_state"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type RuntimeCapabilities struct {
	Available bool
	Details   []string
}

type RuntimeSessionSpec struct {
	ID                SessionID
	Name              string
	StorageAttachment map[string]string
}

type RuntimeSession struct {
	Ref string
}

type RuntimeState struct {
	Observed ObservedState
}

type ExecRequest struct {
	Argv        []string
	Interactive bool
}

type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type StorageCapabilities struct {
	Available bool
	Backend   string
	Shrink    bool
	Compact   bool
	Details   []string
}

type StorageSpec struct {
	ID        string
	SizeBytes int64
}

type StorageHandle struct {
	ID         string
	Attachment map[string]string
}

type StorageState struct {
	Healthy      bool
	Backend      string
	LogicalBytes int64
	UsedBytes    int64
	Details      []string
}

type ShrinkPlan struct {
	HandleID           string
	CurrentBytes       int64
	TargetBytes        int64
	MinimumBytes       int64
	SafetyMarginBytes  int64
	RequiresCompaction bool
	Feasible           bool
	Reason             string
}
