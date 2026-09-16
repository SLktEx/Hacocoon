package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestApprovalCorrelationSurvivesEveryOutcome(t *testing.T) {
	for _, name := range []string{"approved", "denied", "provider-failed", "audit-failed"} {
		t.Run(name, func(t *testing.T) {
			audit := &fakeAudit{}
			provider := &fakeProvider{}
			if name == "provider-failed" {
				provider.err = errors.New("provider failed")
			}
			service := newTestService(t, fakePolicy{evaluation: core.PolicyEvaluation{Decision: core.PolicyRequireApproval}}, nil, audit, provider)
			var prompted string
			result, err := service.RequestWithApproval(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "same-target"}, func(_ context.Context, request core.ApprovalRequest) (bool, error) {
				prompted = request.RequestID
				request.RequestID = "client-cannot-reassign"
				if name == "audit-failed" {
					audit.failAt = len(audit.events) + 1
				}
				return name != "denied", nil
			})
			if (err == nil) != (name == "approved") {
				t.Fatalf("unexpected outcome: %v", err)
			}
			if prompted == "" || result.RequestID != prompted {
				t.Fatalf("prompt=%q result=%q", prompted, result.RequestID)
			}
			for _, event := range audit.events {
				if event.RequestID != prompted {
					t.Fatalf("audit=%q prompt=%q", event.RequestID, prompted)
				}
			}
			if (name == "denied" || name == "audit-failed") && provider.calls != 0 {
				t.Fatal("correlation must not bypass approval or audit")
			}
		})
	}
}
