// Package approvals supplies bounded waiting for trusted human review.
package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	MaxPending        = 128
	MaxPerEnvironment = 16
	MaxPromptBytes    = 16 << 10
	ApprovalLifetime  = 2 * time.Minute
)

type completion struct {
	result core.CapabilityResult
	err    error
}
type entry struct {
	queue     *Queue
	request   core.ApprovalRequest
	ctx       context.Context
	expires   time.Time
	claimed   bool
	decision  chan capability.ApprovalDecision
	completed chan completion
}
type Queue struct {
	mu       sync.Mutex
	entries  map[string]*entry
	lifetime time.Duration
}

func New() *Queue { return &Queue{entries: make(map[string]*entry), lifetime: ApprovalLifetime} }

// A consumer must use BeginApproval so the exact session receives completion.
// Older boolean-only consumers cannot safely use this asynchronous provider.
func (*Queue) Approve(context.Context, core.ApprovalRequest) (bool, error) {
	return false, core.ErrUnsupported
}

func (q *Queue) BeginApproval(ctx context.Context, request core.ApprovalRequest) (capability.ApprovalSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if q == nil || request.RequestID == "" || len(request.RequestID) > 128 {
		return nil, core.ErrInvalidArgument
	}
	request = capability.CloneApprovalRequest(request)
	encoded, err := json.Marshal(request)
	if err != nil || len(encoded) > MaxPromptBytes {
		return nil, core.ErrInvalidArgument
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.entries == nil || q.lifetime <= 0 {
		return nil, core.ErrIncompatibleState
	}
	if _, exists := q.entries[request.RequestID]; exists {
		return nil, core.ErrAlreadyExists
	}
	count := 0
	for _, current := range q.entries {
		if current.request.CapabilityRequest.Environment == request.CapabilityRequest.Environment {
			count++
		}
	}
	if len(q.entries) >= MaxPending || count >= MaxPerEnvironment {
		return nil, core.ErrApprovalDenied
	}
	item := &entry{queue: q, request: request, ctx: ctx, expires: time.Now().Add(q.lifetime), decision: make(chan capability.ApprovalDecision, 1), completed: make(chan completion, 1)}
	q.entries[request.RequestID] = item
	return item, nil
}

func (item *entry) Decide(ctx context.Context) (capability.ApprovalDecision, error) {
	timer := time.NewTimer(time.Until(item.expires))
	defer timer.Stop()
	select {
	case decision := <-item.decision:
		if err := ctx.Err(); err != nil {
			return capability.ApprovalDecision{}, err
		}
		if err := item.ctx.Err(); err != nil {
			return capability.ApprovalDecision{}, err
		}
		if !time.Now().Before(item.expires) {
			return capability.ApprovalDecision{}, context.DeadlineExceeded
		}
		return decision, nil
	case <-ctx.Done():
		return capability.ApprovalDecision{}, ctx.Err()
	case <-item.ctx.Done():
		return capability.ApprovalDecision{}, item.ctx.Err()
	case <-timer.C:
		return capability.ApprovalDecision{}, context.DeadlineExceeded
	}
}

func (q *Queue) PendingApprovals() []core.ApprovalRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]core.ApprovalRequest, 0, len(q.entries))
	for _, item := range q.entries {
		if !item.claimed && item.ctx.Err() == nil && time.Now().Before(item.expires) {
			result = append(result, capability.CloneApprovalRequest(item.request))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RequestID < result[j].RequestID })
	return result
}

// DecideApproval consumes one pending answer and waits for the real application
// outcome. Cancellation after submission does not undo approval or permit replay.
func (q *Queue) DecideApproval(ctx context.Context, id string, decision capability.ApprovalDecision) (core.CapabilityResult, error) {
	q.mu.Lock()
	if err := ctx.Err(); err != nil {
		q.mu.Unlock()
		return core.CapabilityResult{}, err
	}
	item, ok := q.entries[id]
	if !ok || item.claimed || item.ctx.Err() != nil || !time.Now().Before(item.expires) {
		q.mu.Unlock()
		return core.CapabilityResult{}, core.ErrNotFound
	}
	if err := capability.ValidateApprovalDecision(item.request, decision); err != nil {
		q.mu.Unlock()
		return core.CapabilityResult{}, err
	}
	item.claimed = true
	item.decision <- decision
	q.mu.Unlock()
	select {
	case completed := <-item.completed:
		if decision.Save != "" && completed.result.SavedChoice != string(decision.Save) {
			if completed.err != nil {
				return completed.result, completed.err
			}
			return completed.result, core.ErrIncompatibleState
		}
		if !decision.Approved && (errors.Is(completed.err, core.ErrApprovalDenied) || completed.err == core.ErrPolicyDenied) {
			return completed.result, nil
		}
		return completed.result, completed.err
	case <-ctx.Done():
		return core.CapabilityResult{RequestID: id}, ctx.Err()
	}
}

// Complete is bound to this exact admitted session. A late/duplicate completion
// cannot release a replacement entry; delivery never waits for a disconnected UI.
func (item *entry) Complete(result core.CapabilityResult, err error) {
	q := item.queue
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.entries[item.request.RequestID] != item {
		return
	}
	delete(q.entries, item.request.RequestID)
	result.Output = ""
	if result.RequestID != item.request.RequestID {
		err = core.ErrIncompatibleState
	}
	item.completed <- completion{result: result, err: err}
}
