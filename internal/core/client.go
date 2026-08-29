package core

type EnvironmentState string

const (
	EnvironmentRunning EnvironmentState = "running"
	EnvironmentStopped EnvironmentState = "stopped"
	EnvironmentUnknown EnvironmentState = "unknown"
)

type EnvironmentRuntimeStatus struct {
	State EnvironmentState `json:"state"`
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
	HostPort  int
}

type ClientConnection struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	User       string `json:"user,omitempty"`
	Command    string `json:"command,omitempty"`
}
