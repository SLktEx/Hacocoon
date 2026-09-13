package desktopreview

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var ErrNoLongerPending = errors.New("request is no longer pending")

type ReviewExchange interface {
	Exchange(context.Context, Message) (Reply, error)
}
type ToastSurface interface {
	Show(context.Context, string, ToastPage) error
	Remove(context.Context, string) error
}

// ToastManager is used on one native event-loop thread. Decisions use separate
// private peers so a long-running approved operation cannot block other toasts.
// It owns no Policy, durable approval state or provider execution.
type ToastManager struct {
	Exchange  ReviewExchange
	Surface   ToastSurface
	Japanese  bool
	entries   map[string]*ToastFlow
	seen      map[string]bool
	submitted map[string]bool
}

func (m *ToastManager) initialize() error {
	if m.Exchange == nil || m.Surface == nil {
		return ErrInvalid
	}
	if m.entries == nil {
		m.entries = map[string]*ToastFlow{}
		m.seen = map[string]bool{}
		m.submitted = map[string]bool{}
	}
	return nil
}

func (m *ToastManager) Review(ctx context.Context, id string) error {
	if err := m.initialize(); err != nil {
		return err
	}
	if !requestPattern.MatchString(id) {
		return ErrInvalid
	}
	if m.submitted[id] {
		return errors.New("answer already submitted")
	}
	if flow := m.entries[id]; flow != nil {
		return m.show(ctx, flow)
	}
	if len(m.entries) >= 16 {
		return errors.New("notification review capacity reached")
	}
	reply, err := m.Exchange.Exchange(ctx, Message{Action: "select", RequestID: id})
	if err != nil {
		return err
	}
	if reply.Type == "error" && reply.Error == "no_longer_pending" {
		return ErrNoLongerPending
	}
	if reply.Type != "selected" || reply.View == nil || reply.View.Request.RequestID != id {
		return ErrInvalid
	}
	flow, err := NewToastFlow(*reply.View, m.Japanese)
	if err != nil {
		return err
	}
	if err := m.show(ctx, flow); err != nil {
		return err
	}
	m.entries[id], m.seen[id] = flow, true
	return nil
}

func (m *ToastManager) show(ctx context.Context, flow *ToastFlow) error {
	page, err := flow.Page()
	if err != nil {
		return err
	}
	if err := m.Surface.Show(ctx, flow.RequestID(), page); err != nil {
		return err
	}
	return flow.Displayed(page.Nonce)
}

func (m *ToastManager) Refresh(ctx context.Context) error {
	if err := m.initialize(); err != nil {
		return err
	}
	reply, err := m.Exchange.Exchange(ctx, Message{Action: "list"})
	if err != nil {
		return err
	}
	if reply.Type != "pending" {
		return ErrInvalid
	}
	pending := map[string]bool{}
	for _, item := range reply.Pending {
		pending[item.RequestID] = true
	}
	for id := range m.seen {
		if !pending[id] && !m.submitted[id] {
			if m.entries[id] != nil {
				if err := m.Surface.Remove(ctx, id); err != nil {
					return err
				}
			}
			delete(m.entries, id)
			delete(m.seen, id)
		}
	}
	for _, item := range reply.Pending {
		if len(m.entries) >= 16 {
			break
		}
		if !m.seen[item.RequestID] {
			if err := m.Review(ctx, item.RequestID); err != nil && !errors.Is(err, ErrNoLongerPending) {
				return err
			}
			// Native rendering can involve another process. Admit one new
			// notification per refresh so buttons and receipts remain responsive.
			break
		}
	}
	return nil
}

type ToastSubmission struct {
	flow     *ToastFlow
	Intent   ToastIntent
	consumed atomic.Bool
}

func (m *ToastManager) Activate(ctx context.Context, argument string, inputs map[string]string) (*ToastSubmission, error) {
	if m.initialize() != nil {
		return nil, ErrInvalid
	}
	if argument == "" {
		return nil, nil
	}
	nonce, _, ok := strings.Cut(argument, ":")
	if !ok || !hexToken(nonce) {
		return nil, ErrInvalid
	}
	for id, flow := range m.entries {
		if flow.nonce != nonce || m.submitted[id] {
			continue
		}
		intent, err := flow.Activate(argument, inputs)
		if err != nil {
			return nil, err
		}
		if intent == nil {
			return nil, m.show(ctx, flow)
		}
		m.submitted[id] = true
		page := statusToast(m.Japanese, "Sending your explicit answer.\nCompletion is not confirmed yet.", "回答を送信しています。\n操作の完了はまだ未確認です。")
		if err := m.Surface.Show(ctx, id, page); err != nil {
			return nil, err
		}
		return &ToastSubmission{flow: flow, Intent: *intent}, nil
	}
	return nil, ErrInvalid
}

// Run uses a fresh private selection after a user has reviewed the entire saved
// snapshot. Any change discards the intent. No result triggers an automatic retry.
func (s *ToastSubmission) Run(ctx context.Context, peer ReviewExchange) (Reply, error) {
	if s == nil || peer == nil || s.flow == nil || !s.consumed.CompareAndSwap(false, true) {
		return Reply{}, ErrInvalid
	}
	current, err := peer.Exchange(ctx, Message{Action: "select", RequestID: s.Intent.RequestID})
	if err != nil {
		return Reply{}, err
	}
	if current.Type != "selected" || current.View == nil || !s.flow.Matches(*current.View) {
		return Reply{}, ErrInvalid
	}
	approved := s.Intent.Approved
	return peer.Exchange(ctx, Message{Action: "decide", RequestID: s.Intent.RequestID, Token: current.View.Token, Approved: &approved, Save: s.Intent.Save})
}

func (m *ToastManager) Complete(ctx context.Context, submission *ToastSubmission, reply Reply, err error) error {
	if submission == nil || !m.submitted[submission.Intent.RequestID] || m.entries[submission.Intent.RequestID] != submission.flow {
		return ErrInvalid
	}
	id := submission.Intent.RequestID
	confirmed := err == nil && reply.Type == "result" && reply.Error == "" && reply.Result != nil && reply.Result.RequestID == id && reply.Result.SavedChoice == string(submission.Intent.Save)
	if confirmed {
		if submission.Intent.Approved {
			confirmed = reply.Result.ExecutionState == core.CapabilitySucceeded && reply.Result.AuditComplete
		} else {
			confirmed = reply.Result.ExecutionState == core.CapabilityNotExecuted
		}
	}
	page := statusToast(m.Japanese, "The result is unconfirmed.\nCheck Policy and audit first.\nNo automatic retry was made.", "結果は未確認です。\n再試行前に設定と監査を確認。\n自動再送はしていません。")
	// Persistence and execution are independent. A failed provider operation
	// can still have saved the exact reviewed Policy; do not hide that receipt.
	if err == nil && reply.Type == "result" && reply.Result != nil && reply.Result.RequestID == id && reply.Result.ExecutionState == core.CapabilityFailed {
		en, ja := "The operation failed.", "操作は失敗しました。"
		if submission.Intent.Save != "" && reply.Result.SavedChoice == string(submission.Intent.Save) {
			en += "\nThe reviewed Policy was saved."
			ja += "\n確認した設定を保存しました。"
		} else {
			en += "\nCheck current Policy before retry."
			ja += "\n再試行前に現在の設定を確認。"
		}
		page = statusToast(m.Japanese, en+"\nInspect audit; no automatic retry.", ja+"\n監査を確認。自動再送なし。")
	}
	if confirmed {
		en, ja := "Operation denied.", "今回の操作を拒否しました。"
		if submission.Intent.Approved {
			en, ja = "Approved operation completed.", "承認した操作が完了しました。"
		}
		if submission.Intent.Save != "" {
			en += "\nThe reviewed Policy was saved."
			ja += "\n確認した設定を保存しました。"
		} else {
			en += "\nNo Policy was saved."
			ja += "\n設定は保存していません。"
		}
		page = statusToast(m.Japanese, en, ja)
	}
	delete(m.submitted, id)
	delete(m.entries, id)
	return m.Surface.Show(ctx, id, page)
}

func statusToast(ja bool, en, jp string) ToastPage {
	flow := ToastFlow{ja: ja, phase: "confirm"}
	page, err := flow.Page()
	if err != nil {
		return ToastPage{}
	}
	page.Title = flow.word("Hacocoon approval", "Hacocoon 承認結果")
	page.Body = en
	if ja {
		page.Body = jp
	}
	page.Buttons = nil
	page.Inputs = nil
	return page
}
