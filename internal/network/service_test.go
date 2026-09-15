package networkrelay

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const testInstance = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type testAuthority struct {
	mu                 sync.Mutex
	instance, revision string
	decision           core.PolicyDecision
	auditError         error
	events             []core.CapabilityAuditEvent
	onRecord           func()
}

func (a *testAuthority) CurrentInstance(context.Context, string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.instance, nil
}
func (a *testAuthority) PolicyRevision(context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.revision, nil
}
func (a *testAuthority) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return core.PolicyEvaluation{Decision: a.decision}, nil
}
func (a *testAuthority) Record(_ context.Context, e core.CapabilityAuditEvent) error {
	a.mu.Lock()
	a.events = append(a.events, e)
	err, hook := a.auditError, a.onRecord
	a.mu.Unlock()
	if hook != nil && e.Type == "connection-opened" {
		hook()
	}
	return err
}

type testTargets struct{ target Target }

func (t testTargets) Resolve(context.Context, Source, Spec) (Target, error) { return t.target, nil }
func (testTargets) Verify(context.Context, Target) error                    { return nil }

type requesterFunc func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)

func (f requesterFunc) Request(c context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
	return f(c, r)
}
func fixture(t *testing.T, protocol string, port int) (*Service, Spec, *testAuthority) {
	t.Helper()
	a := &testAuthority{instance: testInstance, revision: "r1", decision: core.PolicyAllow}
	spec := Spec{Kind: "host", Target: "fixture", Protocol: protocol, Port: port, DurationSeconds: 10}
	s := &Service{Authority: a, Targets: testTargets{Target{Kind: "host", Name: "fixture", Protocol: protocol, Port: port, Addresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}, Owner: "svc-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, CheckInterval: 5 * time.Millisecond}
	s.Capabilities = requesterFunc(func(ctx context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
		e, _ := a.Evaluate(ctx, r)
		if e.Decision == core.PolicyDeny {
			return core.CapabilityResult{}, core.ErrPolicyDenied
		}
		if _, err := (Provider{}).Execute(ctx, r); err != nil {
			return core.CapabilityResult{}, err
		}
		return core.CapabilityResult{RequestID: "request1", ExecutionState: core.CapabilitySucceeded, AuditComplete: true}, nil
	})
	t.Cleanup(s.Close)
	return s, spec, a
}
func openFixture(t *testing.T, s *Service, spec Spec) *Connection {
	t.Helper()
	c, err := s.Open(context.Background(), Source{"source", testInstance}, spec)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func tcpPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	left, err := net.DialTCP("tcp", nil, l.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatal(err)
	}
	right, err := l.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	_ = left.SetDeadline(deadline)
	_ = right.SetDeadline(deadline)
	t.Cleanup(func() { left.Close(); right.Close() })
	return left, right
}
func TestTCPRelayPreservesResponseAfterClientEOF(t *testing.T) {
	app, client := tcpPair(t)
	up, server := tcpPair(t)
	done := make(chan error, 1)
	go func() { done <- relayTCP(up, client) }()
	response := strings.Repeat("response", 64000)
	go func() {
		body, err := io.ReadAll(server)
		if err == nil && string(body) == "request" {
			_, _ = io.WriteString(server, response)
		}
		server.CloseWrite()
	}()
	_, _ = io.WriteString(app, "request")
	_ = app.CloseWrite()
	got, err := io.ReadAll(app)
	if err != nil || string(got) != response {
		t.Fatalf("response bytes=%d, error=%v", len(got), err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestPolicyDenyNeverDials(t *testing.T) {
	s, spec, a := fixture(t, "tcp", 12345)
	a.decision = core.PolicyDeny
	s.Dial = func(context.Context, string, string) (net.Conn, error) {
		t.Error("denied request reached socket creation")
		return nil, errors.New("dial")
	}
	if _, err := s.Open(context.Background(), Source{"source", testInstance}, spec); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal(err)
	}
}
func TestLiveTCPAuthorityAndLifetimeCloseSocket(t *testing.T) {
	for _, reason := range []string{"revoked", "identity_changed", "policy_expired_or_changed", "expired"} {
		t.Run(reason, func(t *testing.T) {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			port := l.Addr().(*net.TCPAddr).Port
			s, spec, a := fixture(t, "tcp", port)
			if reason == "expired" {
				spec.DurationSeconds = 1
			}
			c := openFixture(t, s, spec)
			peer, err := l.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			_ = peer.SetReadDeadline(time.Now().Add(2 * time.Second))
			switch reason {
			case "revoked":
				if err := s.Revoke(c.Session.ID); err != nil {
					t.Fatal(err)
				}
			case "identity_changed":
				a.mu.Lock()
				a.instance = "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				a.mu.Unlock()
			case "policy_expired_or_changed":
				a.mu.Lock()
				a.revision = "r2"
				a.mu.Unlock()
			}
			n, err := peer.Read(make([]byte, 1))
			if n != 0 || err != io.EOF {
				t.Fatalf("live peer did not close: %d %v", n, err)
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				found := s.List()[0]
				if found.Reason == reason {
					return
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatalf("wrong final observation: %+v", s.List())
		})
	}
}
func TestCancelDuringAuditCannotPublishActive(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	s, spec, a := fixture(t, "tcp", l.Addr().(*net.TCPAddr).Port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.onRecord = cancel
	if c, err := s.Open(ctx, Source{"source", testInstance}, spec); err == nil || c != nil {
		t.Fatal("canceled connection published")
	}
	s.Close()
	if s.List()[0].State == "active" {
		t.Fatal("terminal session resurrected")
	}
}
func TestIncompleteAuditNeverDials(t *testing.T) {
	s, spec, _ := fixture(t, "tcp", 12345)
	s.Capabilities = requesterFunc(func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
		return core.CapabilityResult{ExecutionState: core.CapabilitySucceeded, RequestID: "incomplete"}, nil
	})
	s.Dial = func(context.Context, string, string) (net.Conn, error) { t.Fatal("dial before audit"); return nil, nil }
	if _, err := s.Open(context.Background(), Source{"source", testInstance}, spec); err == nil {
		t.Fatal("missing audit accepted")
	}
}
func TestUDPAssociationEchoAndSinglePeer(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	s, spec, _ := fixture(t, "udp", server.LocalAddr().(*net.UDPAddr).Port)
	c := openFixture(t, s, spec)
	client, relay := tcpPair(t)
	if err := c.BindClient(relay); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { relayDatagrams(c, relay); close(done) }()
	if err := writeDatagram(client, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	_ = server.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 100)
	n, peer, err := server.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "hello" {
		t.Fatalf("%d %v", n, err)
	}
	stranger, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer stranger.Close()
	_, _ = stranger.WriteToUDP([]byte("unrequested"), peer)
	_, _ = server.WriteToUDP([]byte("reply"), peer)
	got, err := readDatagram(client)
	if err != nil || string(got) != "reply" {
		t.Fatalf("unrequested peer or no echo: %q %v", got, err)
	}
	// Zero-byte datagrams remain valid datagrams, not stream EOF.
	_ = writeDatagram(client, nil)
	n, peer, err = server.ReadFromUDP(buf)
	if err != nil || n != 0 {
		t.Fatalf("empty datagram: %d %v", n, err)
	}
	_, _ = server.WriteToUDP(nil, peer)
	got, err = readDatagram(client)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty reply: %d %v", len(got), err)
	}
	if err := s.Revoke(c.Session.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("UDP revoke stuck")
	}
}
func TestUDPIdleCountsBothDirections(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	s, spec, _ := fixture(t, "udp", server.LocalAddr().(*net.UDPAddr).Port)
	c := openFixture(t, s, spec)
	client, relay := tcpPair(t)
	_ = c.BindClient(relay)
	done := make(chan struct{})
	go func() { relayDatagramsWithIdle(c, relay, 200*time.Millisecond); close(done) }()
	for i := 0; i < 15; i++ {
		if err := writeDatagram(client, []byte(strconv.Itoa(i))); err != nil {
			t.Fatal(err)
		}
		_ = server.SetReadDeadline(time.Now().Add(time.Second))
		if _, _, err := server.ReadFromUDP(make([]byte, 50)); err != nil {
			t.Fatal("one-way traffic incorrectly expired:", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle association retained")
	}
	if s.List()[0].Reason != "udp_idle" {
		t.Fatal(s.List())
	}
}
func TestDatagramTruncationAndOversize(t *testing.T) {
	for _, frame := range [][]byte{{0}, {0, 2, 1}, {255, 255}} {
		if _, err := readDatagram(strings.NewReader(string(frame))); err == nil {
			t.Fatalf("accepted malformed frame %v", frame)
		}
	}
	if err := writeDatagram(io.Discard, make([]byte, MaxDatagram+1)); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}

func TestSavedApprovalBindsPostSavePolicyWithoutReusingOtherEdits(t *testing.T) {
	for _, saved := range []bool{true, false} {
		t.Run(strconv.FormatBool(saved), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			s, spec, a := fixture(t, "tcp", listener.Addr().(*net.TCPAddr).Port)
			s.Capabilities = requesterFunc(func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
				a.mu.Lock()
				a.revision = "saved-policy"
				a.mu.Unlock()
				result := core.CapabilityResult{RequestID: "approved", AuditComplete: true, ExecutionState: core.CapabilitySucceeded}
				if saved {
					result.SavedChoice = "allow-env"
				}
				return result, nil
			})
			connection, err := s.Open(context.Background(), Source{"source", testInstance}, spec)
			if saved {
				if err != nil || connection.Session.PolicyRevision != "saved-policy" {
					t.Fatal(connection, err)
				}
				connection.Close()
			} else if !errors.Is(err, errPolicyChanged) || connection != nil {
				t.Fatal("unrelated edit accepted", connection, err)
			}
		})
	}
}
