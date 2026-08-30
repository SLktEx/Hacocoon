package core

// EgressProtocol identifies the application-layer protocol whose hostname is
// being authorized. The first Standard broker supports explicit HTTP proxying
// and HTTPS CONNECT tunnelling; provider-specific network plumbing stays out of
// this contract.
type EgressProtocol string

const (
	EgressHTTP  EgressProtocol = "http"
	EgressHTTPS EgressProtocol = "https"
)

// EgressRequest is the provider-neutral authority request for one outbound
// connection. Host is always a canonical DNS hostname, never an IP literal.
type EgressRequest struct {
	Environment string         `json:"environment"`
	Host        string         `json:"host"`
	Port        int            `json:"port"`
	Protocol    EgressProtocol `json:"protocol"`
}

// EgressGrant is evidence that one normalized outbound connection passed the
// trusted Policy / Approval / Capability boundary. Grants are deliberately
// connection-scoped; the Standard proxy does not turn one approval into an IP
// allowlist or a cross-Environment reusable token.
type EgressGrant struct {
	RequestID   string         `json:"request_id"`
	Environment string         `json:"environment"`
	Host        string         `json:"host"`
	Port        int            `json:"port"`
	Protocol    EgressProtocol `json:"protocol"`
}
