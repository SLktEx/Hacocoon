package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeApprovalClient struct {
	requests   []core.ApprovalRequest
	id         string
	decision   capability.ApprovalDecision
	calls      int
	err        error
	badReceipt bool
}

func (f *fakeApprovalClient) PendingApprovals(context.Context) ([]core.ApprovalRequest, error) {
	return f.requests, nil
}
func (f *fakeApprovalClient) DecideApproval(_ context.Context, id string, decision capability.ApprovalDecision) (core.CapabilityResult, error) {
	f.id, f.decision = id, decision
	f.calls++
	result := core.CapabilityResult{RequestID: id, SavedChoice: string(decision.Save), ExecutionState: core.CapabilityNotExecuted}
	if decision.Approved {
		result.ExecutionState = core.CapabilitySucceeded
		result.AuditComplete = true
	}
	if f.badReceipt {
		result.RequestID = "other"
	}
	return result, f.err
}
func testApprovalRequest(id string) core.ApprovalRequest {
	return core.ApprovalRequest{RequestID: id, CapabilityRequest: core.CapabilityRequest{Capability: "network.egress", Action: "connect", Resource: "example.com"}}
}
func TestApprovalCommandSelectsAndAsksWithoutMandatoryID(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		f := &fakeApprovalClient{requests: []core.ApprovalRequest{testApprovalRequest("wanted")}}
		input := "y\n"
		if multiple {
			f.requests = append([]core.ApprovalRequest{testApprovalRequest("other")}, f.requests...)
			input = "2\ny\n"
		}
		var out, diagnostic bytes.Buffer
		if code := approvalCommand(context.Background(), f, nil, strings.NewReader(input), &out, &diagnostic); code != 0 || f.calls != 1 || f.id != "wanted" || !f.decision.Approved {
			t.Fatalf("selection code=%d id=%s diagnostic=%s", code, f.id, diagnostic.String())
		}
		if !strings.HasPrefix(out.String(), "Approved.\n") || strings.Contains(out.String(), "execution_state") {
			t.Fatal("ordinary approval should be readable without JSON")
		}
	}
}

func TestApprovalCommandJSONReceiptIsOptional(t *testing.T) {
	f := &fakeApprovalClient{requests: []core.ApprovalRequest{testApprovalRequest("request")}}
	var out, diagnostic bytes.Buffer
	if code := approvalCommand(context.Background(), f, []string{"--json"}, strings.NewReader("y\n"), &out, &diagnostic); code != 0 || !strings.Contains(out.String(), `"execution_state":"succeeded"`) {
		t.Fatalf("JSON receipt: %d %s", code, out.String())
	}
}
func TestApprovalCommandSupportsSavedAskAndSeparateCurrentDenial(t *testing.T) {
	f := &fakeApprovalClient{requests: []core.ApprovalRequest{testApprovalRequest("ask")}}
	var out, diagnostic bytes.Buffer
	if code := approvalCommand(context.Background(), f, nil, strings.NewReader("6\nn\n"), &out, &diagnostic); code != 0 || f.decision.Save != capability.AskGlobal || f.decision.Approved {
		t.Fatalf("saved ask: code=%d decision=%+v diagnostic=%s", code, f.decision, diagnostic.String())
	}
}
func TestApprovalCommandListingAndCancellationDoNotDecide(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		input string
	}{{[]string{"--list"}, "y\n"}, {nil, "\n"}, {[]string{"stale"}, "y\n"}} {
		f := &fakeApprovalClient{requests: []core.ApprovalRequest{testApprovalRequest("a"), testApprovalRequest("b")}}
		var out, diagnostic bytes.Buffer
		approvalCommand(context.Background(), f, tc.args, strings.NewReader(tc.input), &out, &diagnostic)
		if f.calls != 0 {
			t.Fatal("listing/cancel/stale triggered approval")
		}
	}
}
func TestApprovalCommandReportsFailureWithoutRetryAndEscapesLabels(t *testing.T) {
	for _, badReceipt := range []bool{false, true} {
		request := testApprovalRequest("a")
		request.CapabilityRequest.Resource = "host\x1b[2J"
		f := &fakeApprovalClient{requests: []core.ApprovalRequest{request}, badReceipt: badReceipt}
		if !badReceipt {
			f.err = errors.New("secret-provider-text")
		}
		var out, diagnostic bytes.Buffer
		if code := approvalCommand(context.Background(), f, nil, strings.NewReader("y\n"), &out, &diagnostic); code != 1 || f.calls != 1 {
			t.Fatal("failed review retried or succeeded")
		}
		if strings.Contains(diagnostic.String(), "\x1b") || strings.Contains(diagnostic.String(), "secret-provider-text") {
			t.Fatal("unsafe display output")
		}
	}
}
