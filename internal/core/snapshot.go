package core

import "time"

// SnapshotSource describes the entire logical aggregate to be captured.
// It is inspection evidence, not a completed snapshot or permission to restore.
type SnapshotSource struct {
	Environment Environment `json:"environment"`
	InstanceID  string      `json:"instance_id"`
}

type SnapshotComponent struct {
	// Binding is an opaque, versioned provider plan. It contains storage identity,
	// never credentials or arbitrary workload configuration, and is compared in CAS.
	Binding   string `json:"binding,omitempty"`
	Role      string `json:"role"`
	NativeRef string `json:"native_ref"`
	Owner     string `json:"owner"`
	State     string `json:"state"`
}
type Snapshot struct {
	// CreatedAt is capture reservation time, not a provider ordering or authority.
	CreatedAt  time.Time           `json:"created_at,omitzero"`
	ID         string              `json:"id"`
	Source     SnapshotSource      `json:"source"`
	State      string              `json:"state"`
	Components []SnapshotComponent `json:"components"`
}
