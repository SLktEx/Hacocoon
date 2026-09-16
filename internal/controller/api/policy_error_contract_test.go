package controlapi

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

type failedReviewService struct {
	err       error
	decisions atomic.Int32
}

func (f *failedReviewService) Pending(context.Context) ([]core.ApprovalRequest, error) {
	return nil, f.err
}
func (f *failedReviewService) Decide(_ context.Context, id string, _ capability.ApprovalDecision) (core.CapabilityResult, error) {
	f.decisions.Add(1)
	return core.CapabilityResult{RequestID: id, ExecutionState: core.CapabilityFailed, Output: "SECRET", Provider: "SECRET"}, f.err
}

func TestReviewFailuresRetainCategoryAndExecutionWithoutBackendText(t *testing.T) {
	for _, tc := range []struct {
		cause error
		code  string
	}{
		{core.ErrAuditIncomplete, "audit_incomplete"}, {core.ErrRecoveryRequired, "recovery_required"},
		{core.ErrCapabilityStale, "stale"}, {core.ErrInvalidArgument, "invalid_argument"},
		{core.ErrNotFound, "not_found"}, {core.ErrIncompatibleState, "incompatible_state"},
		{core.ErrUnsupported, "unsupported"}, {core.ErrPolicyDenied, "denied"},
		{core.ErrApprovalDenied, "denied"}, {context.Canceled, "canceled"},
		{context.DeadlineExceeded, "timeout"}, {errors.New("unknown backend failure"), "internal"},
		{errors.Join(core.ErrNotFound, core.ErrRecoveryRequired, core.ErrAuditIncomplete), "audit_incomplete"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			service := &failedReviewService{err: errors.Join(tc.cause, errors.New("SECRET"))}
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterReviews(s, service); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			pending, pendingErr := client.PendingApprovals(context.Background())
			result, decisionErr := client.DecideApproval(context.Background(), "request-one", capability.ApprovalDecision{Approved: true})
			for _, err := range []error{pendingErr, decisionErr} {
				var status *control.StatusError
				if !errors.As(err, &status) || status.Code != tc.code || strings.Contains(err.Error(), "SECRET") {
					t.Fatal("review lost safe failure category", err)
				}
			}
			if len(pending) != 0 || result.RequestID != "request-one" || result.ExecutionState != core.CapabilityFailed || result.AuditComplete || result.Output != "" || result.Provider != "" || service.decisions.Load() != 1 {
				t.Fatal("review fabricated success, exposed output or repeated decision", pending, result, service.decisions.Load())
			}
		})
	}
}

type failedConfigurationService struct {
	err    error
	writes atomic.Int32
}

func (f *failedConfigurationService) Snapshot(context.Context) (capability.PolicySnapshot, error) {
	return capability.PolicySnapshot{}, f.err
}
func (f *failedConfigurationService) Replace(context.Context, capability.PolicySnapshot) (capability.PolicySnapshot, error) {
	f.writes.Add(1)
	return capability.PolicySnapshot{}, f.err
}

func TestConfigurationErrorsCannotClaimSavedPolicyOrExposeBackendText(t *testing.T) {
	for _, tc := range []struct {
		cause error
		code  string
	}{
		{core.ErrIncompatibleState, "incompatible_state"}, {core.ErrInvalidArgument, "invalid_argument"},
		{core.ErrRecoveryRequired, "recovery_required"}, {core.ErrUnsupported, "unsupported"},
		{errors.New("unknown storage error"), "internal"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			service := &failedConfigurationService{err: errors.Join(tc.cause, errors.New("SECRET"))}
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterConfiguration(s, service); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			read, readErr := client.ReadConfiguration(context.Background())
			write, writeErr := client.ReplaceConfiguration(context.Background(), capability.PolicySnapshot{Revision: "reviewed"})
			for _, err := range []error{readErr, writeErr} {
				var status *control.StatusError
				if !errors.As(err, &status) || status.Code != tc.code || strings.Contains(err.Error(), "SECRET") {
					t.Fatal("configuration failure lost category or leaked backend text", err)
				}
			}
			if read.Revision != "" || write.Revision != "" || len(read.Policy) != 0 || len(write.Policy) != 0 || service.writes.Load() != 1 {
				t.Fatal("unconfirmed Policy was reported or retried", read, write, service.writes.Load())
			}
		})
	}
}
