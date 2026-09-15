package desktopreview

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type reviewFixture struct {
	requests                []core.ApprovalRequest
	decisions               []capability.ApprovalDecision
	pendingErr, decisionErr error
}

func (f *reviewFixture) PendingApprovals(context.Context) ([]core.ApprovalRequest, error) {
	return f.requests, f.pendingErr
}
func (f *reviewFixture) DecideApproval(_ context.Context, id string, d capability.ApprovalDecision) (core.CapabilityResult, error) {
	f.decisions = append(f.decisions, d)
	state := core.CapabilityNotExecuted
	if d.Approved {
		state = core.CapabilitySucceeded
	}
	return core.CapabilityResult{RequestID: id, Provider: "private", Output: "secret provider output", ExecutionState: state, SavedChoice: string(d.Save), AuditComplete: f.decisionErr == nil}, f.decisionErr
}
func newReviewFixture() (*Session, *reviewFixture, string) {
	id := strings.Repeat("a", 32)
	request := core.CapabilityRequest{Environment: "dev", EnvironmentInstance: "env-" + strings.Repeat("b", 32), Capability: "network.egress", Action: "connect", Resource: "example.com:443", Attributes: map[string]string{"hostname": "example.com", "protocol": "tcp", "port": "443"}, Parameters: map[string]string{"secret": "not for UI"}}
	f := &reviewFixture{requests: []core.ApprovalRequest{{RequestID: id, CapabilityRequest: request}}}
	return &Session{Client: f}, f, id
}
func selectReview(t *testing.T, s *Session, id string, seq uint64) *View {
	t.Helper()
	r := s.Handle(context.Background(), Message{Version: 1, Sequence: seq, Action: "select", RequestID: id})
	if r.Type != "selected" || r.View == nil || len(r.View.Token) != 64 {
		t.Fatalf("select: %+v", r)
	}
	return r.View
}
func TestDesktopReviewSelectionAndSavedChoicesUseCommonScope(t *testing.T) {
	for _, choice := range []capability.SavedChoice{"", capability.AllowEnvironment, capability.DenyEnvironment, capability.AskEnvironment, capability.AllowGlobal, capability.DenyGlobal, capability.AskGlobal} {
		t.Run(string(choice), func(t *testing.T) {
			s, f, id := newReviewFixture()
			list := s.Handle(context.Background(), Message{Version: 1, Sequence: 1, Action: "list"})
			if len(list.Pending) != 1 || len(f.decisions) != 0 {
				t.Fatal("listing submitted an answer")
			}
			view := selectReview(t, s, id, 2)
			if view.Request.CapabilityRequest.Parameters != nil || len(view.Options) != 6 {
				t.Fatal("unsafe or incomplete display")
			}
			for _, option := range view.Options {
				if strings.HasSuffix(string(option.Choice), "global") {
					if option.Scope.Environment != "*" || option.Scope.EnvironmentInstance != "" {
						t.Fatal("global scope hidden")
					}
				} else if option.Scope.EnvironmentInstance != f.requests[0].CapabilityRequest.EnvironmentInstance {
					t.Fatal("creation binding lost")
				}
			}
			approved := choice != capability.DenyEnvironment && choice != capability.DenyGlobal
			if choice == capability.AskEnvironment {
				approved = false
			}
			message := Message{Version: 1, Sequence: 3, Action: "decide", RequestID: id, Token: view.Token, Approved: &approved, Save: choice}
			result := s.Handle(context.Background(), message)
			if result.Error != "" || result.Result == nil || len(f.decisions) != 1 || f.decisions[0].Approved != approved || f.decisions[0].Save != choice {
				t.Fatalf("decision: %+v", result)
			}
			if result.Result.Output != "" || result.Result.Provider != "" {
				t.Fatal("provider output leaked")
			}
			message.Sequence++
			if replay := s.Handle(context.Background(), message); replay.Error != "stale_selection" || len(f.decisions) != 1 {
				t.Fatal("replayed decision")
			}
		})
	}
}
func TestDesktopReviewRefusesChangedOrUnavailableDisplayedRequest(t *testing.T) {
	for _, change := range []string{"target", "generation", "saved-scope", "removed", "duplicate", "unavailable", "cancel", "token", "missing-answer", "invalid-choice", "inconsistent-answer"} {
		t.Run(change, func(t *testing.T) {
			s, f, id := newReviewFixture()
			view := selectReview(t, s, id, 1)
			approved := true
			message := Message{Version: 1, Sequence: 2, Action: "decide", RequestID: id, Token: view.Token, Approved: &approved}
			ctx := context.Background()
			switch change {
			case "target":
				f.requests[0].CapabilityRequest.Resource = "other.example:443"
			case "generation":
				f.requests[0].CapabilityRequest.EnvironmentInstance = "env-" + strings.Repeat("c", 32)
			case "saved-scope":
				scope := f.requests[0].CapabilityRequest
				f.requests[0].SavedScope = &scope
			case "removed":
				f.requests = nil
			case "duplicate":
				f.requests = append(f.requests, f.requests[0])
			case "unavailable":
				f.pendingErr = errors.New("private backend failure")
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "token":
				message.Token = strings.Repeat("0", 64)
			case "missing-answer":
				message.Approved = nil
			case "invalid-choice":
				message.Save = "allow-everything"
			case "inconsistent-answer":
				message.Save = capability.DenyGlobal
			}
			r := s.Handle(ctx, message)
			if r.Error == "" || len(f.decisions) != 0 {
				t.Fatalf("accepted %s: %+v", change, r)
			}
		})
	}
}
func TestDesktopReviewFailureReceiptAndSelectionIsolation(t *testing.T) {
	s, f, id := newReviewFixture()
	view := selectReview(t, s, id, 1)
	token := view.Token
	view.Token = "changed"
	view.Request.RequestID = "changed"
	f.decisionErr = errors.New("audit write failed")
	yes := true
	r := s.Handle(context.Background(), Message{Version: 1, Sequence: 2, Action: "decide", RequestID: id, Token: token, Approved: &yes})
	if r.Error != "outcome_unconfirmed" || r.Result == nil || r.Result.AuditComplete || r.Result.ExecutionState != core.CapabilitySucceeded {
		t.Fatalf("lost actual receipt: %+v", r)
	}
}
func TestDesktopReviewProtocolNeverDecidesFromMalformedInputOrEOF(t *testing.T) {
	for _, input := range []string{"", `{"version":1,"sequence":1,"action":"decide","approved":true}`, `{"version":1,"sequence":1,"action":"list","command":"evil"}`, `{"version":1,"sequence":1,"action":"list"} {}`, strings.Repeat("x", 4097)} {
		s, f, _ := newReviewFixture()
		var output bytes.Buffer
		_ = s.Serve(context.Background(), strings.NewReader(input), &output)
		if len(f.decisions) != 0 {
			t.Fatal("protocol submitted a decision")
		}
	}
}
