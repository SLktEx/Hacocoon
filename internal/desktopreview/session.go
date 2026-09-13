package desktopreview

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// ReviewClient uses the existing trusted management endpoint. The session owns
// presentation/correlation only, never Policy, persistence, audit or execution.
type ReviewClient interface {
	PendingApprovals(context.Context) ([]core.ApprovalRequest, error)
	DecideApproval(context.Context, string, capability.ApprovalDecision) (core.CapabilityResult, error)
}

type Message struct {
	Version   int                    `json:"version"`
	Sequence  uint64                 `json:"sequence"`
	Action    string                 `json:"action"`
	RequestID string                 `json:"request_id,omitempty"`
	Token     string                 `json:"token,omitempty"`
	Approved  *bool                  `json:"approved,omitempty"`
	Save      capability.SavedChoice `json:"save,omitempty"`
}

type PendingItem struct {
	RequestID   string `json:"request_id"`
	Capability  string `json:"capability"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Environment string `json:"environment"`
}
type SaveOption struct {
	Choice capability.SavedChoice `json:"choice"`
	Scope  capability.PolicyRule  `json:"scope"`
}
type View struct {
	Request core.ApprovalRequest `json:"request"`
	Token   string               `json:"token"`
	Digest  string               `json:"digest"`
	Options []SaveOption         `json:"options"`
}
type Reply struct {
	Version  int                    `json:"version"`
	Sequence uint64                 `json:"sequence"`
	Type     string                 `json:"type"`
	Pending  []PendingItem          `json:"pending,omitempty"`
	View     *View                  `json:"view,omitempty"`
	Result   *core.CapabilityResult `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

type Session struct {
	Client   ReviewClient
	sequence uint64
	selected *selection
	snapshot []byte
}
type selection struct{ requestID, token string }

func (s *Session) Handle(ctx context.Context, m Message) Reply {
	r := Reply{Version: 1, Sequence: m.Sequence, Type: "error"}
	fail := func(reason string) Reply { r.Pending = nil; r.View = nil; r.Error = reason; return r }
	if s.Client == nil || m.Version != 1 || m.Sequence == 0 || m.Sequence <= s.sequence {
		return fail("invalid_message")
	}
	s.sequence = m.Sequence
	if ctx.Err() != nil {
		return fail("canceled")
	}
	switch m.Action {
	case "list":
		if m.RequestID != "" || m.Token != "" || m.Approved != nil || m.Save != "" {
			return fail("invalid_message")
		}
	case "select":
		if !requestPattern.MatchString(m.RequestID) || m.Token != "" || m.Approved != nil || m.Save != "" {
			return fail("invalid_message")
		}
		s.selected, s.snapshot = nil, nil
	case "decide":
		if s.selected == nil || m.RequestID != s.selected.requestID || m.Token != s.selected.token || m.Approved == nil {
			return fail("stale_selection")
		}
	default:
		return fail("invalid_message")
	}
	// Consume a decision selection before any fallible remote operation. A
	// disconnected, duplicate or failed submission is never automatically retried.
	selected, snapshot := s.selected, s.snapshot
	if m.Action == "decide" {
		s.selected, s.snapshot = nil, nil
	}
	requests, err := s.Client.PendingApprovals(ctx)
	if err != nil {
		return fail("pending_unavailable")
	}
	if len(requests) > 128 {
		return fail("invalid_pending")
	}
	seen := map[string]bool{}
	var current *core.ApprovalRequest
	for _, request := range requests {
		if !requestPattern.MatchString(request.RequestID) || seen[request.RequestID] {
			return fail("invalid_pending")
		}
		seen[request.RequestID] = true
		request = capability.CloneApprovalRequest(request)
		encoded, err := json.Marshal(request)
		if err != nil || len(encoded) > 32<<10 {
			return fail("invalid_pending")
		}
		q := request.CapabilityRequest
		r.Pending = append(r.Pending, PendingItem{request.RequestID, q.Capability, q.Action, q.Resource, q.Environment})
		if request.RequestID == m.RequestID {
			current = &request
		}
	}
	if m.Action == "list" {
		r.Type = "pending"
		return r
	}
	r.Pending = nil
	if current == nil {
		return fail("no_longer_pending")
	}
	encoded, _ := json.Marshal(current)
	if m.Action == "select" {
		var token [32]byte
		if _, err := rand.Read(token[:]); err != nil {
			return fail("selection_unavailable")
		}
		digest := sha256.Sum256(encoded)
		view := &View{Request: *current, Token: hex.EncodeToString(token[:]), Digest: hex.EncodeToString(digest[:])}
		for _, choice := range []capability.SavedChoice{capability.AllowEnvironment, capability.DenyEnvironment, capability.AskEnvironment, capability.AllowGlobal, capability.DenyGlobal, capability.AskGlobal} {
			var rule capability.PolicyRule
			var err error
			if current.SavedScope != nil {
				rule, err = capability.RuleForSavedScope(*current.SavedScope, choice)
			} else {
				rule, err = capability.RuleForSavedChoice(current.CapabilityRequest, choice)
			}
			if err == nil {
				view.Options = append(view.Options, SaveOption{choice, rule})
			}
		}
		s.selected, s.snapshot = &selection{current.RequestID, view.Token}, encoded
		r.Type, r.View = "selected", view
		return r
	}
	if selected == nil || !bytes.Equal(encoded, snapshot) {
		return fail("changed_request")
	}
	decision := capability.ApprovalDecision{Approved: *m.Approved, Save: m.Save}
	if capability.ValidateApprovalDecision(*current, decision) != nil {
		return fail("invalid_decision")
	}
	if ctx.Err() != nil {
		return fail("canceled")
	}
	result, err := s.Client.DecideApproval(ctx, m.RequestID, decision)
	// Preserve actual receipt fields on failure, never provider output/content.
	result.Provider, result.Output = "", ""
	r.Type = "result"
	if result.RequestID == m.RequestID {
		r.Result = &result
	}
	if err != nil || r.Result == nil {
		r.Error = "outcome_unconfirmed"
	}
	return r
}

// Serve uses private local child-process pipes, not the public event bridge.
// EOF/invalid input closes the session without any implicit answer.
func (s *Session) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 4096)
	for scanner.Scan() {
		var message Message
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&message) != nil {
			return errors.New("invalid desktop review message")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return errors.New("invalid desktop review message")
		}
		reply := s.Handle(ctx, message)
		data, err := json.Marshal(reply)
		if err != nil || len(data) > 256<<10 {
			return errors.New("desktop review response exceeds limit")
		}
		if _, err = out.Write(append(data, '\n')); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return scanner.Err()
}
