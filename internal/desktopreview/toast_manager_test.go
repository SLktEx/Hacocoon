package desktopreview

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type sessionExchange struct {
	session *Session
	seq     uint64
}

func (p *sessionExchange) Exchange(ctx context.Context, m Message) (Reply, error) {
	p.seq++
	m.Version, m.Sequence = 1, p.seq
	return p.session.Handle(ctx, m), nil
}

type testToastSurface struct {
	pages map[string]ToastPage
	fail  bool
}

func (s *testToastSurface) Show(_ context.Context, id string, p ToastPage) error {
	if s.fail {
		return errors.New("display unavailable")
	}
	if _, err := ToastXML(p); err != nil {
		return err
	}
	s.pages[id] = p
	return nil
}
func (s *testToastSurface) Remove(_ context.Context, id string) error {
	delete(s.pages, id)
	return nil
}
func testToastManager(t *testing.T) (*ToastManager, *reviewFixture, *testToastSurface, string) {
	t.Helper()
	s, f, id := newReviewFixture()
	surface := &testToastSurface{pages: map[string]ToastPage{}}
	m := &ToastManager{Exchange: &sessionExchange{session: s}, Surface: surface}
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	return m, f, surface, id
}
func toastSubmit(t *testing.T, m *ToastManager, surface *testToastSurface, id string) *ToastSubmission {
	t.Helper()
	ctx := context.Background()
	for m.entries[id].phase == "current" {
		if _, err := m.Activate(ctx, surface.pages[id].Nonce+":next", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Activate(ctx, surface.pages[id].Nonce+":choose", map[string]string{"scope": "once", "policy": "ask"}); err != nil {
		t.Fatal(err)
	}
	job, err := m.Activate(ctx, surface.pages[id].Nonce+":allow", nil)
	if err != nil {
		t.Fatal(err)
	}
	return job
}
func TestToastManagerRefreshExpiryAndMultipleIndependentSubmissions(t *testing.T) {
	m, backend, surface, id := testToastManager(t)
	ctx := context.Background()
	first := surface.pages[id].Nonce
	if err := m.Refresh(ctx); err != nil || surface.pages[id].Nonce != first {
		t.Fatal("poll replaced displayed review", err)
	}
	second := backend.requests[0]
	second.RequestID = strings.Repeat("b", 32)
	backend.requests = append(backend.requests, second)
	if err := m.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	a := toastSubmit(t, m, surface, id)
	b := toastSubmit(t, m, surface, second.RequestID)
	for _, job := range []*ToastSubmission{b, a} {
		result, err := job.Run(ctx, &sessionExchange{session: &Session{Client: backend}})
		if err != nil || result.Error != "" {
			t.Fatal(result, err)
		}
		if err := m.Complete(ctx, job, result, err); err != nil {
			t.Fatal(err)
		}
		if _, err := job.Run(ctx, &sessionExchange{session: &Session{Client: backend}}); err == nil {
			t.Fatal("decision resent")
		}
	}
	if len(backend.decisions) != 2 {
		t.Fatal(backend.decisions)
	}
	backend.requests = nil
	if err := m.Refresh(ctx); err != nil || len(m.entries) != 0 {
		t.Fatal(err)
	}
}
func TestToastManagerCannotDispatchAfterShowFailureOrChangedSnapshot(t *testing.T) {
	for _, failure := range []string{"display", "changed", "lost-response"} {
		t.Run(failure, func(t *testing.T) {
			m, f, surface, id := testToastManager(t)
			ctx := context.Background()
			if failure == "display" {
				surface.fail = true
				if job, err := m.Activate(ctx, surface.pages[id].Nonce+":next", nil); err == nil || job != nil {
					t.Fatal("failed display accepted")
				}
				if len(f.decisions) != 0 {
					t.Fatal("display dispatched")
				}
				return
			}
			job := toastSubmit(t, m, surface, id)
			if failure == "changed" {
				f.requests[0].CapabilityRequest.EnvironmentInstance = "replacement"
			} else {
				f.decisionErr = errors.New("lost response")
			}
			reply, err := job.Run(ctx, &sessionExchange{session: &Session{Client: f}})
			if failure == "changed" && (err == nil || len(f.decisions) != 0) {
				t.Fatal("changed request dispatched")
			}
			if failure == "lost-response" && (reply.Error != "outcome_unconfirmed" || len(f.decisions) != 1) {
				t.Fatal(reply, err)
			}
			if completeErr := m.Complete(ctx, job, reply, err); completeErr != nil {
				t.Fatal(completeErr)
			}
			if !strings.Contains(surface.pages[id].Body, "unconfirmed") {
				t.Fatal(surface.pages[id])
			}
			if _, err := job.Run(ctx, &sessionExchange{session: &Session{Client: f}}); err == nil {
				t.Fatal("unknown answer resent")
			}
		})
	}
}
func TestToastManagerRemovesExpiredUnansweredReview(t *testing.T) {
	m, f, surface, id := testToastManager(t)
	old := surface.pages[id].Nonce
	f.requests = nil
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(surface.pages) != 0 {
		t.Fatal("expired toast remains")
	}
	if job, err := m.Activate(context.Background(), old+":allow", nil); job != nil || err == nil {
		t.Fatal("expired answer accepted")
	}
}
func TestToastManagerFailedOperationShowsKnownSavedPolicy(t *testing.T) {
	m, _, surface, id := testToastManager(t)
	job := toastSubmit(t, m, surface, id)
	job.Intent.Save = "allow-environment"
	r := Reply{Type: "result", Result: &core.CapabilityResult{RequestID: id, SavedChoice: "allow-environment", ExecutionState: core.CapabilityFailed}}
	if err := m.Complete(context.Background(), job, r, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(surface.pages[id].Body, "operation failed") || !strings.Contains(surface.pages[id].Body, "Policy was saved") {
		t.Fatal(surface.pages[id])
	}
}
