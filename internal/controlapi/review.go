package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const MethodApprovalPending = "approval.pending"
const MethodApprovalDecide = "approval.decide"

type reviewService interface {
	Pending(context.Context) ([]core.ApprovalRequest, error)
	Decide(context.Context, string, capability.ApprovalDecision) (core.CapabilityResult, error)
}
type reviewDecisionRequest struct {
	RequestID string                 `json:"request_id"`
	Approved  *bool                  `json:"approved"`
	Save      capability.SavedChoice `json:"save,omitempty"`
}
type reviewResponse struct {
	Result core.CapabilityResult `json:"result"`
	Error  *responseStatus       `json:"error,omitempty"`
}

// RegisterReviews belongs only on the existing trusted management socket.
// Neither guest Git sockets nor the read-only HTTP event bridge register it.
func RegisterReviews(server *control.Server, service reviewService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodApprovalPending, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if len(payload) != 0 && string(payload) != "null" && string(payload) != "{}" {
			return nil, control.ErrInvalidArgument
		}
		requests, err := service.Pending(ctx)
		if err != nil {
			return nil, responseError(reviewStatus(err))
		}
		result := make([]ApprovalRequestPayload, 0, len(requests))
		for _, request := range requests {
			result = append(result, approvalPayload(capability.CloneApprovalRequest(request)))
		}
		return result, nil
	}); err != nil {
		return err
	}
	return server.Register(MethodApprovalDecide, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request reviewDecisionRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.Approved == nil || request.RequestID == "" || strings.TrimSpace(request.RequestID) != request.RequestID || len(request.RequestID) > 128 {
			return nil, control.ErrInvalidArgument
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		result, err := service.Decide(ctx, request.RequestID, capability.ApprovalDecision{Approved: *request.Approved, Save: request.Save})
		result.Output = ""
		result.Provider = ""
		return reviewResponse{Result: result, Error: reviewStatus(err)}, nil
	})
}

// Review clients receive stable error categories, never provider output or
// arbitrary errors that may contain credentials, signed URLs or guest text.
func reviewStatus(err error) *responseStatus {
	if err == nil {
		return nil
	}
	code := "internal"
	switch {
	case errors.Is(err, core.ErrAuditIncomplete):
		code = "audit_incomplete"
	case errors.Is(err, core.ErrRecoveryRequired):
		code = "recovery_required"
	case errors.Is(err, core.ErrCapabilityStale):
		code = "stale"
	case errors.Is(err, core.ErrInvalidArgument):
		code = "invalid_argument"
	case errors.Is(err, core.ErrNotFound):
		code = "not_found"
	case errors.Is(err, core.ErrIncompatibleState):
		code = "incompatible_state"
	case errors.Is(err, core.ErrUnsupported):
		code = "unsupported"
	case errors.Is(err, core.ErrPolicyDenied), errors.Is(err, core.ErrApprovalDenied):
		code = "denied"
	case errors.Is(err, context.Canceled):
		code = "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	}
	return &responseStatus{Code: code, Message: "approval review did not complete successfully; inspect the request and Policy before retrying"}
}

func (c *Client) PendingApprovals(ctx context.Context) ([]core.ApprovalRequest, error) {
	var payloads []ApprovalRequestPayload
	if err := c.wire.Call(ctx, MethodApprovalPending, nil, &payloads); err != nil {
		return nil, err
	}
	result := make([]core.ApprovalRequest, 0, len(payloads))
	for _, payload := range payloads {
		result = append(result, capability.CloneApprovalRequest(payload.coreRequest()))
	}
	return result, nil
}

func (c *Client) DecideApproval(ctx context.Context, id string, decision capability.ApprovalDecision) (core.CapabilityResult, error) {
	var response reviewResponse
	err := c.wire.Call(ctx, MethodApprovalDecide, reviewDecisionRequest{RequestID: id, Approved: &decision.Approved, Save: decision.Save}, &response)
	if err != nil {
		return core.CapabilityResult{RequestID: id}, err
	}
	err = responseError(response.Error)
	if err == nil && (response.Result.RequestID != id || response.Result.SavedChoice != string(decision.Save) ||
		(decision.Approved && (response.Result.ExecutionState != core.CapabilitySucceeded || !response.Result.AuditComplete)) ||
		(!decision.Approved && response.Result.ExecutionState != core.CapabilityNotExecuted)) {
		err = core.ErrIncompatibleState
	}
	return response.Result, err
}
