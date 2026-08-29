package core

import "time"

type PolicyDecision string

const (
	PolicyAllow           PolicyDecision = "allow"
	PolicyDeny            PolicyDecision = "deny"
	PolicyRequireApproval PolicyDecision = "require-approval"
)

type CapabilityRequest struct {
	Capability  string            `json:"capability"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Parameters  map[string]string `json:"-"`
}

type PolicyEvaluation struct {
	Decision PolicyDecision `json:"decision"`
	Reason   string         `json:"reason,omitempty"`
}

type ApprovalRequest struct {
	CapabilityRequest CapabilityRequest
	Reason            string
}

type CapabilityResult struct {
	Provider string `json:"provider"`
	Output   string `json:"output,omitempty"`
}

type CapabilityAuditEvent struct {
	Time        time.Time         `json:"time"`
	Type        string            `json:"type"`
	Capability  string            `json:"capability"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Decision    PolicyDecision    `json:"decision,omitempty"`
	Approved    *bool             `json:"approved,omitempty"`
	Success     *bool             `json:"success,omitempty"`
	Reason      string            `json:"reason,omitempty"`
}
