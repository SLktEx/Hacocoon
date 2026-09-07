package review

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type source struct {
	requests []core.ApprovalRequest
	calls    int
	id       string
}

func (s *source) PendingApprovals() []core.ApprovalRequest { return s.requests }
func (s *source) DecideApproval(_ context.Context, id string, _ capability.ApprovalDecision) (core.CapabilityResult, error) {
	s.calls++
	s.id = id
	return core.CapabilityResult{RequestID: id}, nil
}
func TestReviewRoutesExactRequestAndNeverAmbiguousIdentity(t *testing.T) {
	a := &source{requests: []core.ApprovalRequest{{RequestID: "a", CapabilityRequest: core.CapabilityRequest{Attributes: map[string]string{"host": "first"}}}}}
	b := &source{requests: []core.ApprovalRequest{{RequestID: "b"}}}
	s := New(a, b)
	pending, err := s.Pending(context.Background())
	if err != nil || len(pending) != 2 {
		t.Fatal(err)
	}
	pending[0].CapabilityRequest.Attributes["host"] = "changed"
	if a.requests[0].CapabilityRequest.Attributes["host"] != "first" {
		t.Fatal("view mutated source")
	}
	if _, err := s.Decide(context.Background(), "b", capability.ApprovalDecision{}); err != nil || b.id != "b" || a.calls != 0 {
		t.Fatal("wrong source decided")
	}
	b.requests[0].RequestID = "a"
	if _, err := s.Decide(context.Background(), "a", capability.ApprovalDecision{}); !errors.Is(err, core.ErrIncompatibleState) || a.calls != 0 || b.calls != 1 {
		t.Fatal("ambiguous ID selected a source")
	}
}
func TestReviewRefusesStaleAndCanceledWithoutSubmitting(t *testing.T) {
	a := &source{requests: []core.ApprovalRequest{{RequestID: "a"}}}
	s := New(a)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Decide(ctx, "a", capability.ApprovalDecision{}); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled review accepted")
	}
	if _, err := s.Decide(context.Background(), "old", capability.ApprovalDecision{}); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("stale review accepted")
	}
	if a.calls != 0 {
		t.Fatal("request was submitted")
	}
}
