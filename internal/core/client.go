package core

type EnvironmentState string

const (
	EnvironmentRunning EnvironmentState = "running"
	EnvironmentStopped EnvironmentState = "stopped"
	EnvironmentUnknown EnvironmentState = "unknown"
)

type EnvironmentRuntimeStatus struct {
	// Absent is a complete provider observation, distinct from an unknown
	// runtime state or failed inspection. Keep the public state JSON unchanged.
	Absent bool             `json:"-"`
	State  EnvironmentState `json:"state"`
}

type EnvironmentStatus struct {
	Environment Environment      `json:"environment"`
	State       EnvironmentState `json:"state"`
}

type LocalPortRequest struct {
	Protocol   string
	HostPort   int
	TargetPort int
}

type SSHAccessRequest struct {
	PublicKey string
	HostPort  int // zero requests automatic selection by the runtime authority
}

type ClientConnection struct {
	HostPublicKey string `json:"host_public_key,omitempty"`
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	TargetPort    int    `json:"target_port"`
	User          string `json:"user,omitempty"`
	Command       string `json:"command,omitempty"`
}
