package core

import "time"

type PolicyDecision string

const (
	PolicyAllow           PolicyDecision = "allow"
	PolicyDeny            PolicyDecision = "deny"
	PolicyRequireApproval PolicyDecision = "require-approval"
)

type CapabilityExecutionState string

const (
	CapabilityNotExecuted CapabilityExecutionState = "not-executed"
	CapabilitySucceeded   CapabilityExecutionState = "succeeded"
	CapabilityFailed      CapabilityExecutionState = "failed"
)

type CapabilityRequest struct {
	EnvironmentInstance string            `json:"environment_instance,omitempty"`
	Capability          string            `json:"capability"`
	Action              string            `json:"action"`
	Resource            string            `json:"resource,omitempty"`
	Environment         string            `json:"environment,omitempty"`
	Attributes          map[string]string `json:"attributes,omitempty"`
	Parameters          map[string]string `json:"-"`
}

type PolicyEvaluation struct {
	Decision PolicyDecision `json:"decision"`
	Reason   string         `json:"reason,omitempty"`
}

type ApprovalRequest struct {
	SavedScope *CapabilityRequest

	CapabilityRequest CapabilityRequest
	Reason            string
}

type CapabilityResult struct {
	SavedChoice string `json:"saved_choice,omitempty"`

	Provider       string                   `json:"provider"`
	Output         string                   `json:"output,omitempty"`
	RequestID      string                   `json:"request_id,omitempty"`
	ExecutionState CapabilityExecutionState `json:"execution_state,omitempty"`
	AuditComplete  bool                     `json:"audit_complete"`
}

type CapabilityAuditEvent struct {
	SavedScope *CapabilityRequest `json:"saved_scope,omitempty"`

	EnvironmentInstance string            `json:"environment_instance,omitempty"`
	SavedChoice         string            `json:"saved_choice,omitempty"`
	Time                time.Time         `json:"time"`
	RequestID           string            `json:"request_id"`
	Type                string            `json:"type"`
	Capability          string            `json:"capability"`
	Action              string            `json:"action"`
	Resource            string            `json:"resource,omitempty"`
	Environment         string            `json:"environment,omitempty"`
	Attributes          map[string]string `json:"attributes,omitempty"`
	Decision            PolicyDecision    `json:"decision,omitempty"`
	Approved            *bool             `json:"approved,omitempty"`
	Success             *bool             `json:"success,omitempty"`
	Reason              string            `json:"reason,omitempty"`
}
