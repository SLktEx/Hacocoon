package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/review"
	"github.com/SLktEx/Hacocoon/modules/standard/approvals"
)

type reviewWireFixture struct {
	calls      atomic.Int32
	badReceipt bool
}

func (*reviewWireFixture) Pending(context.Context) ([]core.ApprovalRequest, error) {
	return []core.ApprovalRequest{{RequestID: "request-one", CapabilityRequest: core.CapabilityRequest{Capability: "test", Parameters: map[string]string{"credential": "SECRET"}}}}, nil
}
func (f *reviewWireFixture) Decide(_ context.Context, id string, _ capability.ApprovalDecision) (core.CapabilityResult, error) {
	f.calls.Add(1)
	result := core.CapabilityResult{RequestID: id, ExecutionState: core.CapabilitySucceeded, Output: "SECRET", Provider: "SECRET"}
	if f.badReceipt {
		result.RequestID = "other"
		return result, nil
	}
	return result, errors.Join(core.ErrAuditIncomplete, errors.New("SECRET"))
}

func TestReviewWireBoundsInputAndPreservesSanitizedFailureReceipt(t *testing.T) {
	f := &reviewWireFixture{}
	path := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterReviews(server, f); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var raw json.RawMessage
	if err := client.wire.Call(ctx, MethodApprovalPending, nil, &raw); err != nil || strings.Contains(string(raw), "SECRET") {
		t.Fatalf("pending exposed parameters: %s %v", raw, err)
	}
	for _, payload := range []string{`null`, `{}`, `{"request_id":"request-one"}`, `{"request_id":"request-one","approved":true,"resource":"replace"}`, `{"request_id":"request-one","approved":true} {}`} {
		if err := client.wire.Call(ctx, MethodApprovalDecide, json.RawMessage(payload), nil); err == nil {
			t.Fatalf("invalid decision accepted: %s", payload)
		}
	}
	if f.calls.Load() != 0 {
		t.Fatal("malformed decision reached service")
	}
	result, err := client.DecideApproval(ctx, "request-one", capability.ApprovalDecision{Approved: true})
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "audit_incomplete" || result.ExecutionState != core.CapabilitySucceeded || result.RequestID != "request-one" {
		t.Fatalf("failure lost outcome: %+v %v", result, err)
	}
	if strings.Contains(err.Error(), "SECRET") || result.Output != "" || result.Provider != "" {
		t.Fatal("review leaked provider output")
	}
}

type reviewAskPolicy struct{}

func (reviewAskPolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: core.PolicyRequireApproval}, nil
}

type reviewAudit struct{}

func (reviewAudit) Record(context.Context, core.CapabilityAuditEvent) error { return nil }

type reviewProvider struct{ calls atomic.Int32 }

func (*reviewProvider) Capability() string { return "network.egress" }
func (p *reviewProvider) Execute(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
	p.calls.Add(1)
	return core.CapabilityResult{}, nil
}

func TestReviewWireRunsActualQueuedServiceAndRejectsReplay(t *testing.T) {
	queue := approvals.New()
	provider := &reviewProvider{}
	service, err := capability.New(reviewAskPolicy{}, queue, reviewAudit{}, provider)
	if err != nil {
		t.Fatal(err)
	}
	path := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterReviews(server, review.New(queue)); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := service.Request(ctx, core.CapabilityRequest{Capability: "network.egress", Action: "connect", Resource: "example.com"})
		done <- err
	}()
	var id string
	for id == "" {
		requests, err := client.PendingApprovals(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) == 1 {
			id = requests[0].RequestID
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("request did not appear")
		case <-time.After(time.Millisecond):
		}
	}
	result, err := client.DecideApproval(ctx, id, capability.ApprovalDecision{Approved: true})
	if err != nil || !result.AuditComplete || result.ExecutionState != core.CapabilitySucceeded || result.RequestID != id {
		t.Fatalf("wire outcome: %+v %v", result, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := client.DecideApproval(ctx, id, capability.ApprovalDecision{Approved: true}); err == nil {
		t.Fatal("wire replay accepted")
	}
	if provider.calls.Load() != 1 {
		t.Fatal("duplicate execution")
	}
}

func TestReviewClientRefusesMismatchedReceipt(t *testing.T) {
	path := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterReviews(server, &reviewWireFixture{badReceipt: true}); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.DecideApproval(context.Background(), "request-one", capability.ApprovalDecision{Approved: true}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("mismatched receipt was successful")
	}
}
