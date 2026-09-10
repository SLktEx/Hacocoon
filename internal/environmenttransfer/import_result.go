package environmenttransfer

// DefaultPayloadLimit is the trusted product transfer budget.
const DefaultPayloadLimit int64 = 64 << 30

type ImportResult struct {
	Environment string   `json:"environment"`
	Workspace   string   `json:"workspace,omitempty"`
	OCI         string   `json:"oci,omitempty"`
	State       string   `json:"state"`
	Offline     []string `json:"offline,omitempty"`
}
