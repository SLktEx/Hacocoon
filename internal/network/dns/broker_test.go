package nameresolution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type requestFunc func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)

func (f requestFunc) Request(ctx context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	return f(ctx, req)
}
func TestBrokerRejectsUntrustedCapabilityResults(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		audit        bool
		state        core.CapabilityExecutionState
	}{
		{"missing audit", "[]", false, core.CapabilitySucceeded},
		{"not executed", "[]", true, ""},
		{"malformed", "garbage", true, core.CapabilitySucceeded},
		{"invalid address", "[\"bad\"]", true, core.CapabilitySucceeded},
		{"scoped address", "[\"fe80::1%eth0\"]", true, core.CapabilitySucceeded},
		{"too many addresses", "[" + strings.Repeat("\"10.0.0.1\",", 32) + "\"10.0.0.1\"]", true, core.CapabilitySucceeded},
		{"oversize", strings.Repeat(" ", 4097), true, core.CapabilitySucceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broker := New(requestFunc(func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
				return core.CapabilityResult{AuditComplete: tc.audit, ExecutionState: tc.state, Output: tc.output}, nil
			}))
			if _, err := broker.Resolve(context.Background(), "dev", "example.com"); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("accepted untrusted result: %v", err)
			}
		})
	}
}
