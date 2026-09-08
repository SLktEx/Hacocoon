package approvals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type askPolicy struct{}

func (askPolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: core.PolicyRequireApproval}, nil
}

type auditSink struct{ failType string }

func (a auditSink) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	if event.Type == a.failType {
		return errors.New("audit unavailable")
	}
	return nil
}

type echoProvider struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (*echoProvider) Capability() string { return "local.echo" }
func (p *echoProvider) Execute(ctx context.Context, _ core.CapabilityRequest) (core.CapabilityResult, error) {
	p.calls.Add(1)
	if p.started != nil {
		close(p.started)
	}
	if p.release != nil {
		select {
		case <-p.release:
		case <-ctx.Done():
			return core.CapabilityResult{}, ctx.Err()
		}
	}
	return core.CapabilityResult{Output: "must-not-reach-review"}, nil
}
func request() core.CapabilityRequest {
	return core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "example.com", Attributes: map[string]string{"port": "443"}}
}
func newService(t *testing.T, q *Queue, p *echoProvider, audit auditSink) *capability.Service {
	t.Helper()
	s, err := capability.New(askPolicy{}, q, audit, p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func pending(t *testing.T, q *Queue) core.ApprovalRequest {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if requests := q.PendingApprovals(); len(requests) == 1 {
			return requests[0]
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("approval was not queued")
		}
	}
}
func receive(t *testing.T, ch <-chan completion) completion {
	t.Helper()
	select {
	case c := <-ch:
		return c
	case <-time.After(3 * time.Second):
		t.Fatal("operation did not complete")
		return completion{}
	}
}
func start(ctx context.Context, s *capability.Service) <-chan completion {
	done := make(chan completion, 1)
	go func() { r, err := s.Request(ctx, request()); done <- completion{r, err} }()
	return done
}

func TestQueueWaitsForActualCompletionAndConsumesOnlyOnce(t *testing.T) {
	q := New()
	p := &echoProvider{started: make(chan struct{}), release: make(chan struct{})}
	s := newService(t, q, p, auditSink{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := start(ctx, s)
	prompt := pending(t, q)
	prompt.CapabilityRequest.Attributes["port"] = "9999"
	prompt.SavedScope.Attributes["port"] = "*"
	if got := pending(t, q); got.CapabilityRequest.Attributes["port"] != "443" || got.SavedScope.Attributes["port"] != "443" {
		t.Fatal("display snapshot changed stored authority")
	}
	reviewed := make(chan completion, 1)
	go func() {
		r, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true})
		reviewed <- completion{r, err}
	}()
	select {
	case <-p.started:
	case <-ctx.Done():
		t.Fatal("provider did not start")
	}
	if len(q.PendingApprovals()) != 0 {
		t.Fatal("claimed request still offered")
	}
	if _, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("replay: %v", err)
	}
	select {
	case <-reviewed:
		t.Fatal("queue acknowledgement masqueraded as completion")
	default:
	}
	close(p.release)
	r := receive(t, reviewed)
	original := receive(t, done)
	if r.err != nil || original.err != nil || r.result.RequestID != prompt.RequestID || !r.result.AuditComplete || r.result.ExecutionState != core.CapabilitySucceeded || r.result.Output != "" || p.calls.Load() != 1 {
		t.Fatalf("review result: %+v original: %+v", r, original)
	}
}

func TestQueueCancellationExpiryAndAuditFailure(t *testing.T) {
	for _, mode := range []string{"cancel", "expire", "approval-audit", "completion-audit"} {
		t.Run(mode, func(t *testing.T) {
			q := New()
			if mode == "expire" {
				q.lifetime = 30 * time.Millisecond
			}
			p := &echoProvider{}
			audit := auditSink{}
			if mode == "approval-audit" {
				audit.failType = "approval-decision"
			}
			if mode == "completion-audit" {
				audit.failType = "completed"
			}
			s := newService(t, q, p, audit)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := start(ctx, s)
			prompt := pending(t, q)
			if mode == "cancel" {
				cancel()
			}
			if strings.Contains(mode, "audit") {
				result, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true})
				if err == nil {
					t.Fatal("audit failure acknowledged")
				}
				if mode == "completion-audit" && (!errors.Is(err, core.ErrAuditIncomplete) || result.ExecutionState != core.CapabilitySucceeded) {
					t.Fatalf("lost executed but unaudited state: %+v %v", result, err)
				}
			}
			if r := receive(t, done); r.err == nil {
				t.Fatal("operation unexpectedly succeeded")
			}
			if len(q.PendingApprovals()) != 0 || len(q.entries) != 0 {
				t.Fatal("session was not released")
			}
			if mode != "completion-audit" && p.calls.Load() != 0 {
				t.Fatal("failed approval executed")
			}
			if _, err := q.DecideApproval(context.Background(), prompt.RequestID, capability.ApprovalDecision{Approved: true}); !errors.Is(err, core.ErrNotFound) {
				t.Fatal("expired request replayed")
			}
		})
	}
}

func TestInvalidAndCanceledDecisionsDoNotConsumePending(t *testing.T) {
	q := New()
	s := newService(t, q, &echoProvider{}, auditSink{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := start(ctx, s)
	prompt := pending(t, q)
	for _, choice := range []capability.SavedChoice{"unknown", capability.DenyGlobal} {
		if _, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true, Save: choice}); err == nil {
			t.Fatal("invalid saved choice consumed request")
		}
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := q.DecideApproval(canceled, prompt.RequestID, capability.ApprovalDecision{Approved: true}); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled review accepted")
	}
	if len(q.PendingApprovals()) != 1 {
		t.Fatal("failed decision removed request")
	}
	result, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{})
	if err != nil || result.ExecutionState != core.CapabilityNotExecuted {
		t.Fatalf("one-shot denial: %+v %v", result, err)
	}
	if r := receive(t, done); !errors.Is(r.err, core.ErrApprovalDenied) {
		t.Fatal("request did not observe denial")
	}
}

func TestQueueBoundsAndExactSessionOwnership(t *testing.T) {
	q := New()
	ctx := context.Background()
	var sessions []capability.ApprovalSession
	for i := 0; i < MaxPending; i++ {
		r := request()
		r.Environment = fmt.Sprintf("env-%d", i/MaxPerEnvironment)
		session, err := q.BeginApproval(ctx, core.ApprovalRequest{RequestID: fmt.Sprint(i), CapabilityRequest: r})
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, session)
	}
	if _, err := q.BeginApproval(ctx, core.ApprovalRequest{RequestID: "overflow", CapabilityRequest: request()}); !errors.Is(err, core.ErrApprovalDenied) {
		t.Fatal("total limit not enforced")
	}
	for i, session := range sessions {
		session.Complete(core.CapabilityResult{RequestID: fmt.Sprint(i)}, nil)
	}
	sessions = nil
	for i := 0; i < MaxPerEnvironment; i++ {
		session, err := q.BeginApproval(ctx, core.ApprovalRequest{RequestID: fmt.Sprint(i), CapabilityRequest: request()})
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, session)
	}
	if _, err := q.BeginApproval(ctx, core.ApprovalRequest{RequestID: "overflow", CapabilityRequest: request()}); !errors.Is(err, core.ErrApprovalDenied) {
		t.Fatal("per-Environment limit not enforced")
	}
	if _, err := q.BeginApproval(ctx, core.ApprovalRequest{RequestID: "0", CapabilityRequest: request()}); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatal("duplicate request was admitted")
	}
	sessions[0].Complete(core.CapabilityResult{RequestID: "0"}, nil)
	replacement, err := q.BeginApproval(ctx, core.ApprovalRequest{RequestID: "0", CapabilityRequest: request()})
	if err != nil {
		t.Fatal(err)
	}
	sessions[0].Complete(core.CapabilityResult{RequestID: "0"}, nil)
	if len(q.entries) != MaxPerEnvironment {
		t.Fatal("stale completion released replacement")
	}
	replacement.Complete(core.CapabilityResult{RequestID: "0"}, nil)
	for i, session := range sessions[1:] {
		session.Complete(core.CapabilityResult{RequestID: fmt.Sprint(i + 1)}, nil)
	}
	if _, err := q.BeginApproval(ctx, core.ApprovalRequest{RequestID: "huge", Reason: strings.Repeat("x", MaxPromptBytes)}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal("oversized prompt accepted")
	}
}

func TestConcurrentReviewersExecuteOnlyOnce(t *testing.T) {
	q := New()
	p := &echoProvider{}
	s := newService(t, q, p, auditSink{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := start(ctx, s)
	prompt := pending(t, q)
	gate := make(chan struct{})
	results := make(chan completion, 20)
	for i := 0; i < cap(results); i++ {
		go func() {
			<-gate
			r, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true})
			results <- completion{r, err}
		}()
	}
	close(gate)
	accepted := 0
	for i := 0; i < cap(results); i++ {
		r := receive(t, results)
		if r.err == nil {
			accepted++
		} else if !errors.Is(r.err, core.ErrNotFound) {
			t.Fatal(r.err)
		}
	}
	if accepted != 1 || p.calls.Load() != 1 {
		t.Fatalf("accepted=%d executed=%d", accepted, p.calls.Load())
	}
	if r := receive(t, done); r.err != nil {
		t.Fatal(r.err)
	}
}

func TestReviewCancellationAfterSubmissionDoesNotUndoOrBlockCompletion(t *testing.T) {
	q := New()
	p := &echoProvider{started: make(chan struct{}), release: make(chan struct{})}
	s := newService(t, q, p, auditSink{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := start(ctx, s)
	prompt := pending(t, q)
	reviewCtx, stopReview := context.WithCancel(ctx)
	defer stopReview()
	reviewed := make(chan completion, 1)
	go func() {
		r, err := q.DecideApproval(reviewCtx, prompt.RequestID, capability.ApprovalDecision{Approved: true})
		reviewed <- completion{r, err}
	}()
	select {
	case <-p.started:
	case <-ctx.Done():
		t.Fatal("provider did not start")
	}
	stopReview()
	if r := receive(t, reviewed); !errors.Is(r.err, context.Canceled) || r.result.RequestID != prompt.RequestID || r.result.ExecutionState != "" {
		t.Fatalf("ambiguous cancellation lost: %+v", r)
	}
	if _, err := q.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{}); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("cancellation permitted a second decision")
	}
	close(p.release)
	if r := receive(t, done); r.err != nil || r.result.ExecutionState != core.CapabilitySucceeded {
		t.Fatal("disconnected reviewer prevented completion")
	}
	if len(q.entries) != 0 {
		t.Fatal("completed session retained")
	}
}
