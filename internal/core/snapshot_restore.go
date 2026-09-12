package core

// SnapshotRestore owns independent copies and the current target identity.
// Preparation never publishes an Environment or changes current data.
type SnapshotRestore struct {
	ID    string   `json:"id"`
	Saved Snapshot `json:"saved"`
	// Before preserves schema-8 backup ownership only; new operations leave it empty.
	Before     Snapshot            `json:"before,omitzero"`
	Current    SnapshotSource      `json:"current"`
	State      string              `json:"state"`
	Components []SnapshotComponent `json:"components"`
}
