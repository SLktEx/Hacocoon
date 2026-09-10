package environmenttransfer

type ImportResult struct {
	Environment string   `json:"environment"`
	Workspace   string   `json:"workspace,omitempty"`
	OCI         string   `json:"oci,omitempty"`
	State       string   `json:"state"`
	Offline     []string `json:"offline,omitempty"`
}
