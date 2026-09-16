package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

type approvalResponseOutcome struct {
	approved bool
	err      error
}
type approvalResponseService struct{ finished chan approvalResponseOutcome }

func (s *approvalResponseService) RequestWithApproval(ctx context.Context, request core.CapabilityRequest, approve func(context.Context, core.ApprovalRequest) (bool, error)) (core.CapabilityResult, error) {
	approved, err := approve(ctx, core.ApprovalRequest{RequestID: "request-one", CapabilityRequest: request})
	s.finished <- approvalResponseOutcome{approved, err}
	result := core.CapabilityResult{RequestID: "request-one", ExecutionState: core.CapabilityNotExecuted}
	if approved && err == nil {
		result.ExecutionState = core.CapabilitySucceeded
	}
	return result, err
}

func TestApprovalResponsesCannotTurnTransportFailureIntoConsent(t *testing.T) {
	for _, mode := range []string{"unknown-frame", "broken-json", "stdin-closed", "legacy-save", "approved"} {
		t.Run(mode, func(t *testing.T) {
			service := &approvalResponseService{finished: make(chan approvalResponseOutcome, 1)}
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterGeneral(s, fakeBases{}, fakeRunner{}, fakeEvents{}, service); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, err := client.wire.OpenStream(ctx, MethodCapabilityRequest, CapabilityRequestPayload{Capability: "local.echo", Action: "echo"})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(conn)
			var prompt capabilityServerFrame
			if err := decoder.Decode(&prompt); err != nil || prompt.Type != capabilityFrameApproval || prompt.Approval == nil || prompt.Approval.RequestID != "request-one" || prompt.SavedChoices {
				t.Fatal("approval contract was not established", prompt, err)
			}
			switch mode {
			case "stdin-closed":
				half, ok := conn.(interface{ CloseWrite() error })
				if !ok {
					t.Fatal("approval stream has no input EOF")
				}
				err = half.CloseWrite()
			case "broken-json":
				_, err = io.WriteString(conn, "{broken}\n")
			default:
				frame := capabilityClientFrame{Type: capabilityFrameApprovalResponse, Approved: true}
				if mode == "unknown-frame" {
					frame.Type = "unverified-consent"
				}
				if mode == "legacy-save" {
					frame.Save = capability.AllowGlobal
				}
				err = json.NewEncoder(conn).Encode(frame)
			}
			if err != nil {
				t.Fatal(err)
			}
			var outcome approvalResponseOutcome
			select {
			case outcome = <-service.finished:
			case <-ctx.Done():
				t.Fatal("approval exchange did not finish", ctx.Err())
			}
			var terminal capabilityServerFrame
			readErr := decoder.Decode(&terminal)
			switch mode {
			case "approved":
				if !outcome.approved || outcome.err != nil || readErr != nil || terminal.Result == nil || terminal.Result.ExecutionState != core.CapabilitySucceeded {
					t.Fatal("explicit approval was lost", outcome, terminal, readErr)
				}
			case "legacy-save":
				if outcome.approved || !errors.Is(outcome.err, core.ErrUnsupported) || readErr != nil || terminal.Result == nil || terminal.Result.ExecutionState != core.CapabilityNotExecuted || terminal.Error == nil || terminal.Error.Code != "unsupported" {
					t.Fatal("saved approval was silently downgraded", outcome, terminal, readErr)
				}
			default:
				var transportError *capabilityStreamTransportError
				if outcome.approved || !errors.As(outcome.err, &transportError) || !errors.Is(readErr, io.EOF) || terminal.Result != nil {
					t.Fatal("transport failure became consent or a receipt", outcome, terminal, readErr)
				}
				if mode == "unknown-frame" && !errors.Is(outcome.err, control.ErrProtocol) {
					t.Fatal("protocol cause lost", outcome.err)
				}
				if mode == "stdin-closed" && !errors.Is(outcome.err, io.EOF) {
					t.Fatal("input EOF cause lost", outcome.err)
				}
			}
		})
	}
}
