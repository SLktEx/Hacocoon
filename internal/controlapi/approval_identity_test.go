package controlapi

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

func TestApprovalPayloadPreservesTrustedEnvironmentIdentity(t *testing.T) {
	id, _ := core.NewEnvironmentInstanceID()
	request := core.ApprovalRequest{CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Environment: "dev", EnvironmentInstance: id}}
	if got := approvalPayload(request).coreRequest(); got.CapabilityRequest.EnvironmentInstance != id {
		t.Fatal("approval lost creation identity")
	}
	// Ordinary capability clients cannot assert a controller-owned identity.
	if got := capabilityPayload(request.CapabilityRequest).coreRequest(); got.EnvironmentInstance != "" {
		t.Fatal("client can assert trusted creation identity")
	}
}
