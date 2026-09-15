package core

type ObservedState string

const (
	ObservedUnknown ObservedState = "unknown"
	ObservedRunning ObservedState = "running"
	ObservedStopped ObservedState = "stopped"
	ObservedError   ObservedState = "error"
)

type RuntimeCapabilities struct {
	Available bool
	Details   []string
}

type RuntimePrepareSpec struct {
	StorageAttachment map[string]string
}

type RuntimeState struct {
	Observed ObservedState
}

type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
