package networkrelay

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type sourceFixture struct{}

func (sourceFixture) ResolveEnvironmentInstance(context.Context, net.IP) (string, string, error) {
	return "source", testInstance, nil
}
func testHTTP(t *testing.T, s *Service) string {
	t.Helper()
	h := &Handler{Service: s, Sources: sourceFixture{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This component fixture substitutes only the trusted ingress observer;
		// real Incus acceptance separately checks the actual source guard.
		r.RemoteAddr = "192.0.2.2:31000"
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://")
}
func TestGuestHTTPRoundTripAndPolicyDenial(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	s, spec, a := fixture(t, "tcp", l.Addr().(*net.TCPAddr).Port)
	endpoint := testHTTP(t, s)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()
	c, view, err := openGuestAt(context.Background(), endpoint, spec)
	if err != nil {
		t.Fatal(err)
	}
	if view.Source.Instance != testInstance || view.Target.Owner != "svc-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatal(view)
	}
	_, _ = c.Write([]byte("payload"))
	got := make([]byte, 7)
	if _, err := io.ReadFull(c, got); err != nil || string(got) != "payload" {
		t.Fatal(string(got), err)
	}
	c.Close()
	a.mu.Lock()
	a.decision = core.PolicyDeny
	a.mu.Unlock()
	if _, _, err := openGuestAt(context.Background(), endpoint, spec); err == nil || !strings.Contains(err.Error(), "policy_denied") {
		t.Fatal(err)
	}
}
func TestHTTPRejectsUnmanagedAndInjectedSource(t *testing.T) {
	s, _, _ := fixture(t, "tcp", 12345)
	h := &Handler{Service: s, Sources: sourceFixture{}}
	for _, body := range []string{
		`{"kind":"host","target":"fixture","protocol":"tcp","port":12345,"duration_seconds":10,"environment":"other"}`,
		`{"kind":"host","target":"fixture","protocol":"tcp","port":12345,"duration_seconds":10}{}`,
	} {
		req := httptest.NewRequest("POST", Path, strings.NewReader(body))
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", upgrade)
		req.RemoteAddr = "192.0.2.2:33000"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatal(w.Code)
		}
	}
	req := httptest.NewRequest("POST", Path, strings.NewReader("{}"))
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", upgrade)
	req.RemoteAddr = "127.0.0.1:33000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatal(w.Code)
	}
	if len(s.List()) != 0 {
		t.Fatal("invalid source reserved a session")
	}
}
func TestGuestRefusalAndCancellationAreBounded(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	s, spec, _ := fixture(t, "tcp", port)
	if _, _, err := openGuestAt(context.Background(), testHTTP(t, s), spec); err == nil || !strings.Contains(err.Error(), "connection_refused") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := openGuestAt(ctx, "127.0.0.1:1", spec); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestPendingRevocationNeverDials(t *testing.T) {
	s, spec, _ := fixture(t, "udp", 31111)
	pending := make(chan struct{})
	s.Capabilities = requesterFunc(func(ctx context.Context, _ core.CapabilityRequest) (core.CapabilityResult, error) {
		close(pending)
		<-ctx.Done()
		return core.CapabilityResult{}, ctx.Err()
	})
	s.Dial = func(context.Context, string, string) (net.Conn, error) {
		t.Error("revoked pending request dialed")
		return nil, core.ErrPolicyDenied
	}
	done := make(chan error, 1)
	go func() { _, err := s.Open(context.Background(), Source{"source", testInstance}, spec); done <- err }()
	<-pending
	all := s.List()
	if len(all) != 1 || all[0].State != "pending" {
		t.Fatal(all)
	}
	if err := s.Revoke(all[0].ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("revoked pending request succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("pending request stuck")
	}
}
