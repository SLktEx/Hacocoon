package core

// WorkloadSpec describes an OCI application workload owned by one Hacocoon
// Environment. Workloads are runtime resources; they are not nested containerd
// containers inside the Environment.
type WorkloadSpec struct {
	Environment          string            `json:"environment"`
	Name                 string            `json:"name"`
	Image                string            `json:"image"`
	Command              []string          `json:"command,omitempty"`
	EnvironmentVariables map[string]string `json:"environment_variables,omitempty"`
	Ephemeral            bool              `json:"ephemeral,omitempty"`
}

// Workload is the controller-visible identity of a Hacocoon-managed OCI
// application container.
type Workload struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	RuntimeRef  string `json:"runtime_ref"`
	Image       string `json:"image"`
	State       string `json:"state"`
	Ephemeral   bool   `json:"ephemeral,omitempty"`
}
