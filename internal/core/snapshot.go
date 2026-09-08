package core

// SnapshotSource describes the entire logical aggregate to be captured.
// It is inspection evidence, not a completed snapshot or permission to restore.
type SnapshotSource struct {
	Environment Environment `json:"environment"`
	InstanceID  string      `json:"instance_id"`
}
