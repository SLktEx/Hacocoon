package core

// StreamTarget is a public, durable binding, not a credential. The controller
// resolves all provider authority and checks the live access grant on every use.
type StreamTarget struct {
	Environment string              `json:"environment"`
	Instance    string              `json:"instance"`
	Workspace   WorkspaceID         `json:"workspace"`
	AccessMode  WorkspaceAccessMode `json:"access_mode"`
	Service     string              `json:"service"`
	Grant       string              `json:"grant"`
}

func (t StreamTarget) Valid() bool {
	return t.Environment != "" && ValidEnvironmentInstanceID(t.Instance) && t.Workspace != "" &&
		(t.AccessMode == WorkspaceReadOnly || t.AccessMode == WorkspaceReadWrite) && t.Service == "ssh" && t.Grant != ""
}
