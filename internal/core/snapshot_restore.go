package core

// SnapshotRestore owns preparation of a replacement aggregate. Before is a
// completed pre-restore backup; preparation never authorizes replacing it or
// publishing a runnable Environment. Those require a canonical lifecycle commit.
type SnapshotRestore struct {
	ID         string              `json:"id"`
	Saved      Snapshot            `json:"saved"`
	Before     Snapshot            `json:"before"`
	State      string              `json:"state"`
	Components []SnapshotComponent `json:"components"`
}
