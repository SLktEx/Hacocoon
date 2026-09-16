package networkrelay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestSessionHistoryEvictsOldestTerminalRecordAndPreservesActiveConnection(t *testing.T) {
	s, spec, authority := fixture(t, "tcp", 443)
	upstream, peer := net.Pipe()
	t.Cleanup(func() { _ = peer.Close() })
	s.Dial = func(context.Context, string, string) (net.Conn, error) { return upstream, nil }
	live := openFixture(t, s, spec)
	defer live.Close()
	// The first association retains its approved Policy. Subsequent requests
	// are individually denied without changing that live association's Policy.
	s.Capabilities = requesterFunc(func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
		return core.CapabilityResult{}, core.ErrApprovalDenied
	})
	terminalIDs := make([]string, 0, 1024)
	seen := map[string]bool{live.Session.ID: true}
	for i := 0; i < 1024; i++ {
		if conn, err := s.Open(context.Background(), Source{"source", testInstance}, spec); conn != nil || !errors.Is(err, core.ErrApprovalDenied) {
			t.Fatal(conn, err)
		}
		authority.mu.Lock()
		last := authority.events[len(authority.events)-1]
		authority.mu.Unlock()
		id := last.Attributes["connection_id"]
		if last.Type != "connection-closed" || last.Reason != "approval_denied" || id == "" || seen[id] {
			t.Fatal("denial lost its unique terminal audit", last)
		}
		seen[id] = true
		terminalIDs = append(terminalIDs, id)
	}
	listed := s.List()
	if len(listed) != 1024 {
		t.Fatal("session history was not bounded", len(listed))
	}
	remaining := make(map[string]Session, len(listed))
	for _, view := range listed {
		remaining[view.ID] = view
	}
	if remaining[live.Session.ID].State != "active" {
		authority.mu.Lock()
		defer authority.mu.Unlock()
		for _, event := range authority.events {
			if event.Type == "connection-closed" && event.Attributes["connection_id"] == live.Session.ID {
				t.Fatalf("live connection closed before history check: %s", event.Reason)
			}
		}
		t.Fatal("history eviction removed live authority")
	}
	if _, ok := remaining[terminalIDs[0]]; ok {
		t.Fatal("oldest terminal record was retained")
	}
	for _, id := range terminalIDs[1:] {
		view, ok := remaining[id]
		if !ok || view.State != "failed" || view.Reason != "approval_denied" {
			t.Fatal("newer denial record lost", id, view)
		}
	}
	if err := s.Revoke(terminalIDs[0]); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("evicted history remained addressable", err)
	}
	if err := live.Validate(); err != nil {
		t.Fatal("eviction invalidated active connection", err)
	}
}

func TestPendingSessionLimitsApplyPerSourceAndAcrossController(t *testing.T) {
	s, spec, _ := fixture(t, "tcp", 443)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{}, 128)
	completed := make(chan error, 128)
	s.Capabilities = requesterFunc(func(ctx context.Context, _ core.CapabilityRequest) (core.CapabilityResult, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return core.CapabilityResult{}, ctx.Err()
	})
	s.Dial = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("pending approval reached dial")
	}
	for i := 0; i < 128; i++ {
		source := Source{Environment: fmt.Sprintf("source-%d", i/16), Instance: testInstance}
		go func() {
			conn, err := s.Open(ctx, source, spec)
			if conn != nil {
				conn.Close()
				err = errors.New("pending request acquired a connection")
			}
			completed <- err
		}()
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("pending request did not enter approval")
		}
		if i == 15 {
			if conn, err := s.Open(ctx, source, spec); conn != nil || !errors.Is(err, core.ErrStorageBusy) {
				t.Fatal("per-source quota bypassed", conn, err)
			}
		}
	}
	if conn, err := s.Open(ctx, Source{"another-source", testInstance}, spec); conn != nil || !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("controller-wide quota bypassed", conn, err)
	}
	if len(s.List()) != 128 {
		t.Fatal("refused request reserved a session", len(s.List()))
	}
	cancel()
	for i := 0; i < 128; i++ {
		if err := waitClientResult(t, completed); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	for _, view := range s.List() {
		if view.State != "closed" && view.State != "failed" {
			t.Fatal("cancellation retained pending authority", view)
		}
	}
}

type resolutionFailure struct{ err error }

func (f resolutionFailure) Resolve(context.Context, Source, Spec) (Target, error) {
	return Target{}, f.err
}
func (f resolutionFailure) Verify(context.Context, Target) error { return f.err }

func TestFailedResolutionRecordsBoundedReasonWithoutBackendDetails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		reason string
	}{
		{"missing", core.ErrNotFound, "target_not_found"},
		{"dns", &net.DNSError{Err: "private resolver output", Name: "private.invalid"}, "dns_failed"},
		{"invalid", core.ErrInvalidArgument, "invalid_request"},
		{"unknown", errors.New("credential=secret-token"), "connection_failed"},
		{"timeout", context.DeadlineExceeded, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, spec, a := fixture(t, "tcp", 443)
			s.Targets = resolutionFailure{tc.err}
			if conn, err := s.Open(context.Background(), Source{"source", testInstance}, spec); conn != nil || !errors.Is(err, tc.err) {
				t.Fatal(conn, err)
			}
			views := s.List()
			if len(views) != 1 || views[0].State != "failed" || views[0].Reason != tc.reason {
				t.Fatal(views)
			}
			a.mu.Lock()
			defer a.mu.Unlock()
			if len(a.events) != 1 || a.events[0].Reason != tc.reason || a.events[0].Type != "connection-closed" {
				t.Fatal("terminal audit exposed backend failure", a.events)
			}
		})
	}
}

type revisionFailure struct{ Authority }

func (revisionFailure) PolicyRevision(context.Context) (string, error) {
	return "", core.ErrRuntimeUnavailable
}

func TestSessionOpenRejectsIncompleteOrMismatchedAuthorityBeforeDial(t *testing.T) {
	for _, mode := range []string{"unavailable", "invalid-spec", "invalid-source", "stale-source", "wrong-target", "empty-addresses", "unreadable-policy", "nil-upstream"} {
		t.Run(mode, func(t *testing.T) {
			s, spec, a := fixture(t, "tcp", 443)
			source := Source{"source", testInstance}
			want := core.ErrInvalidArgument
			dials := 0
			s.Dial = func(context.Context, string, string) (net.Conn, error) { dials++; return nil, nil }
			switch mode {
			case "unavailable":
				s.Capabilities = nil
				want = core.ErrPolicyDenied
			case "invalid-spec":
				spec.DurationSeconds = 0
			case "invalid-source":
				source.Environment = "-option"
			case "stale-source":
				a.instance = "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				want = core.ErrCapabilityStale
			case "wrong-target":
				target := s.Targets.(testTargets)
				target.target.Name = "other"
				s.Targets = target
				want = core.ErrIncompatibleState
			case "empty-addresses":
				target := s.Targets.(testTargets)
				target.target.Addresses = nil
				s.Targets = target
			case "unreadable-policy":
				s.Authority = revisionFailure{s.Authority}
				want = core.ErrPolicyDenied
			case "nil-upstream":
				want = core.ErrIncompatibleState
			}
			conn, err := s.Open(context.Background(), source, spec)
			if conn != nil || !errors.Is(err, want) {
				t.Fatal("incomplete authority was accepted", conn, err)
			}
			wantDials := 0
			if mode == "nil-upstream" {
				wantDials = 1
			}
			if dials != wantDials {
				t.Fatal("authority failure reached provider", dials)
			}
			for _, view := range s.List() {
				if view.State != "failed" {
					t.Fatal("failed open retained live authority", view)
				}
			}
		})
	}
}

func TestFailedConnectionCannotBeReboundOrOverwriteItsTerminalAudit(t *testing.T) {
	s, spec, a := fixture(t, "tcp", 443)
	upstream, peer := net.Pipe()
	defer func() { _ = peer.Close() }()
	s.Dial = func(context.Context, string, string) (net.Conn, error) { return upstream, nil }
	conn := openFixture(t, s, spec)
	view := s.List()[0]
	view.Target.Addresses[0] = view.Target.Addresses[0].Next()
	if s.List()[0].Target.Addresses[0] != conn.Session.Target.Addresses[0] {
		t.Fatal("observation mutated pinned destination")
	}
	conn.Fail("transport_failed")
	client, remote := net.Pipe()
	defer func() { _ = client.Close(); _ = remote.Close() }()
	if err := remote.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := conn.BindClient(client); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("terminal authority rebound", err)
	}
	var b [1]byte
	if _, err := remote.Read(b[:]); !errors.Is(err, io.EOF) {
		t.Fatal("refused client was retained", err)
	}
	conn.Close()
	if err := s.Revoke(conn.Session.ID); err != nil {
		t.Fatal(err)
	}
	view = s.List()[0]
	if view.State != "failed" || view.Reason != "transport_failed" {
		t.Fatal("cleanup replaced original failure", view)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.events) != 2 || a.events[1].Reason != "transport_failed" {
		t.Fatal("duplicate cleanup changed terminal audit", a.events)
	}
}
