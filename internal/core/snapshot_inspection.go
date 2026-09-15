package core

// SnapshotInspection is read-only evidence, never a deletion capability.
// Details name persisted storage identities, not configuration or credentials.
type SnapshotInspection struct {
	ID          string                        `json:"id"`
	Environment string                        `json:"environment"`
	State       string                        `json:"state"`
	Partial     bool                          `json:"partial"`
	Components  []SnapshotComponentInspection `json:"components"`
}

type SnapshotComponentInspection struct {
	Role       string `json:"role"`
	State      string `json:"state"`
	Provider   string `json:"provider,omitempty"`
	Project    string `json:"project,omitempty"`
	Pool       string `json:"pool,omitempty"`
	Object     string `json:"object,omitempty"`
	Presence   string `json:"presence"`
	Check      string `json:"check"`
	References *int   `json:"references,omitempty"`
	// Backing is explicitly uninspected: provider metadata does not prove disk health.
	Backing string `json:"backing"`
}
