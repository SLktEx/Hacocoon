package controlapi

import (
	"context"
	"errors"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type savedWireCapabilities struct {
	fakeCapabilities
	decision capabilityapp.ApprovalDecision
}

func (f *savedWireCapabilities) RequestWithDecision(ctx context.Context, r core.CapabilityRequest, decide func(context.Context, core.ApprovalRequest) (capabilityapp.ApprovalDecision, error)) (core.CapabilityResult, error) {
	d, err := decide(ctx, core.ApprovalRequest{CapabilityRequest: r})
	f.decision = d
	return core.CapabilityResult{Provider: "demo"}, err
}
func TestSavedApprovalCrossesControllerStream(t *testing.T) {
	service := &savedWireCapabilities{}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterGeneral(s, fakeBases{}, fakeRunner{}, fakeEvents{}, service); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	_, err := client.RequestCapabilityWithDecision(context.Background(), core.CapabilityRequest{Capability: "demo", Action: "approve"}, func(context.Context, core.ApprovalRequest) (capabilityapp.ApprovalDecision, error) {
		return capabilityapp.ApprovalDecision{Approved: true, Save: capabilityapp.AllowGlobal}, nil
	})
	if err != nil || service.decision.Save != capabilityapp.AllowGlobal || !service.decision.Approved {
		t.Fatalf("%#v %v", service.decision, err)
	}
}
func TestSavedApprovalRejectsNonSupportingPeer(t *testing.T) {
	service := &fakeCapabilities{}
	client, cancel := startGeneralControlAPITestServer(t, service)
	defer cancel()
	_, err := client.RequestCapabilityWithDecision(context.Background(), core.CapabilityRequest{Capability: "demo", Action: "approve"}, func(context.Context, core.ApprovalRequest) (capabilityapp.ApprovalDecision, error) {
		return capabilityapp.ApprovalDecision{Approved: true, Save: capabilityapp.AllowGlobal}, nil
	})
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("save silently downgraded: %v", err)
	}
}
