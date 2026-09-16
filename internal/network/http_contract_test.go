package networkrelay

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func connectionRequest(t *testing.T, spec Spec) *http.Request {
	t.Helper()
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, Path, bytes.NewReader(data))
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", upgrade)
	r.RemoteAddr = "192.0.2.2:33000"
	return r
}

type unmanagedSource struct{}

func (unmanagedSource) ResolveEnvironmentInstance(context.Context, net.IP) (string, string, error) {
	return "", "", core.ErrNotFound
}

func TestHTTPRefusesUnsupportedTransportAndUntrustedRequestBeforeAuthorization(t *testing.T) {
	for _, mode := range []string{"unavailable", "method", "path", "absolute-url", "empty", "oversized", "upgrade", "connection", "source-address", "unknown-source", "no-hijacker"} {
		t.Run(mode, func(t *testing.T) {
			s, spec, _ := fixture(t, "tcp", 443)
			h := &Handler{Service: s, Sources: sourceFixture{}}
			r := connectionRequest(t, spec)
			want := http.StatusBadRequest
			switch mode {
			case "unavailable":
				h.Service = nil
				want = http.StatusServiceUnavailable
			case "method":
				r.Method = http.MethodGet
			case "path":
				r.RequestURI = Path + "?environment=other"
			case "absolute-url":
				r.URL.Scheme = "http"
				r.URL.Host = "host.invalid"
			case "empty":
				r.ContentLength = 0
			case "oversized":
				r.ContentLength = 4097
			case "upgrade":
				r.Header.Set("Upgrade", "other")
			case "connection":
				r.Header.Set("Connection", "keep-alive")
			case "source-address":
				r.RemoteAddr = "untrusted"
				want = http.StatusForbidden
			case "unknown-source":
				h.Sources = unmanagedSource{}
				want = http.StatusForbidden
			case "no-hijacker":
				want = http.StatusServiceUnavailable
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != want || len(s.List()) != 0 {
				t.Fatal("unsupported transport acquired connection authority", w.Code, s.List())
			}
		})
	}
}

type failedHijack struct{ *httptest.ResponseRecorder }

func (failedHijack) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, core.ErrRuntimeUnavailable
}

func TestHTTPHijackFailureClosesAuthorizedUpstreamAndRecordsFailure(t *testing.T) {
	s, spec, a := fixture(t, "tcp", 443)
	upstream, peer := net.Pipe()
	defer func() { _ = peer.Close() }()
	s.Dial = func(context.Context, string, string) (net.Conn, error) { return upstream, nil }
	h := &Handler{Service: s, Sources: sourceFixture{}}
	h.ServeHTTP(failedHijack{httptest.NewRecorder()}, connectionRequest(t, spec))
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	var b [1]byte
	if n, err := peer.Read(b[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("hijack failure left upstream usable", n, err)
	}
	views := s.List()
	if len(views) != 1 || views[0].State != "failed" || views[0].Reason != "transport_failed" {
		t.Fatal(views)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.events) != 2 || a.events[0].Type != "connection-opened" || a.events[1].Type != "connection-closed" || a.events[1].Reason != "transport_failed" {
		t.Fatal("transport failure lost its audit", a.events)
	}
}

func TestGuestUDPUpgradeCarriesOnlyFramedDatagrams(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	s, spec, _ := fixture(t, "udp", server.LocalAddr().(*net.UDPAddr).Port)
	conn, view, err := openGuestAt(context.Background(), testHTTP(t, s), spec)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if view.Target.Protocol != "udp" {
		t.Fatal(view)
	}
	if err := writeDatagram(conn, []byte("request")); err != nil {
		t.Fatal(err)
	}
	if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var b [32]byte
	n, peer, err := server.ReadFromUDP(b[:])
	if err != nil || string(b[:n]) != "request" {
		t.Fatal("framing leaked into datagram", string(b[:n]), err)
	}
	if _, err := server.WriteToUDP([]byte("response"), peer); err != nil {
		t.Fatal(err)
	}
	got, err := readDatagram(conn)
	if err != nil || string(got) != "response" {
		t.Fatal(string(got), err)
	}
	if err := conn.(*guestConn).CloseWrite(); err != nil {
		t.Fatal("guest lost TCP half-close support", err)
	}
}
