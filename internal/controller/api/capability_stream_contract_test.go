package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/policy"
)

func TestCapabilityClientRefusesMalformedFramesBeforePublishingResult(t *testing.T) {
	for _, mode := range []string{"eof", "invalid-json", "approval-missing", "approval-with-result", "approval-with-error", "result-missing", "result-with-approval", "unknown", "callback-failure", "unsupported-save", "execution-failure"} {
		t.Run(mode, func(t *testing.T) {
			approval := ApprovalRequestPayload{RequestID: "approval", Capability: "local.echo", Action: "echo"}
			result := core.CapabilityResult{RequestID: "receipt", ExecutionState: core.CapabilityFailed, AuditComplete: true}
			frame := capabilityServerFrame{Type: capabilityFrameApproval, Approval: &approval}
			switch mode {
			case "approval-missing":
				frame.Approval = nil
			case "approval-with-result":
				frame.Result = &result
			case "approval-with-error":
				frame.Error = &responseStatus{Code: "denied", Message: "refused"}
			case "result-missing":
				frame = capabilityServerFrame{Type: capabilityFrameResult}
			case "result-with-approval":
				frame.Type = capabilityFrameResult
				frame.Result = &result
			case "unknown":
				frame.Type = "execute-arbitrary"
			case "execution-failure":
				frame = capabilityServerFrame{Type: capabilityFrameResult, Result: &result, Error: &responseStatus{Code: "internal", Message: "operation failed", ExitCode: 7}}
			}
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := s.RegisterStream(MethodCapabilityRequest, func(context.Context, json.RawMessage) (control.Stream, error) {
					return func(_ context.Context, conn net.Conn) error {
						if mode == "eof" {
							return nil
						}
						if mode == "invalid-json" {
							_, err := io.WriteString(conn, "{broken}\n")
							return err
						}
						return json.NewEncoder(conn).Encode(frame)
					}, nil
				}); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			callbackError := errors.New("approval UI failed")
			got, err := client.RequestCapabilityWithDecision(context.Background(), core.CapabilityRequest{Capability: "local.echo", Action: "echo"}, func(context.Context, core.ApprovalRequest) (capabilityapp.ApprovalDecision, error) {
				calls++
				if mode == "callback-failure" {
					return capabilityapp.ApprovalDecision{}, callbackError
				}
				return capabilityapp.ApprovalDecision{Approved: true, Save: capabilityapp.SavedChoice("allow-env")}, nil
			})
			if err == nil {
				t.Fatal("invalid/failed stream reported success", got)
			}
			wantCalls := 0
			if mode == "callback-failure" || mode == "unsupported-save" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatal("untrusted frame reached approval callback", calls)
			}
			switch mode {
			case "callback-failure":
				if !errors.Is(err, callbackError) {
					t.Fatal(err)
				}
			case "unsupported-save":
				if !errors.Is(err, core.ErrUnsupported) {
					t.Fatal(err)
				}
			case "execution-failure":
				var exit interface{ ExitCode() int }
				var status *control.StatusError
				if !errors.As(err, &exit) || exit.ExitCode() != 7 || !errors.As(err, &status) || status.Code != "internal" || got.RequestID != "receipt" || !got.AuditComplete || !strings.Contains(err.Error(), "operation failed") {
					t.Fatal("execution failure lost receipt/status", got, err)
				}
			default:
				if mode != "invalid-json" && !errors.Is(err, control.ErrProtocol) {
					t.Fatal("invalid frame lacked protocol error", err)
				}
			}
			if mode != "execution-failure" && got.RequestID != "" {
				t.Fatal("invalid frame published receipt", got)
			}
		})
	}
}
