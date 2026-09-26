package egressproxy

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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func TestPinnedAddressAdmission(t *testing.T) {
	for _, address := range []string{
		"10.0.0.8", "172.16.0.1", "172.31.255.254", "192.168.1.1",
		"fc00::1", "fd12::1", "::ffff:192.168.1.1", "100.64.0.1",
		"198.18.0.1", "93.184.216.34", "2606:4700:4700::1111",
	} {
		t.Run("allow/"+address, func(t *testing.T) {
			proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
			proxy.resolver = fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP(address)}}}
			got, err := proxy.resolvePinned(context.Background(), "example.com")
			if err != nil || len(got) != 1 || got[0].String() != net.ParseIP(address).String() {
				t.Fatalf("addresses=%v err=%v", got, err)
			}
		})
	}
	for _, address := range []string{
		"127.0.0.1", "127.1.2.3", "::1", "::ffff:127.0.0.1",
		"0.0.0.0", "0.1.2.3", "::", "169.254.169.254", "fe80::1",
		"224.0.0.1", "ff02::1", "255.255.255.255", "240.0.0.1", "invalid",
	} {
		for _, mixed := range []string{"alone", "first", "last"} {
			t.Run("reject/"+address+"/"+mixed, func(t *testing.T) {
				answers := []net.IPAddr{{IP: net.ParseIP(address)}}
				allowed := []net.IPAddr{{IP: net.ParseIP("10.0.0.8")}, {IP: net.ParseIP("93.184.216.34")}}
				if mixed == "first" {
					answers = append(answers, allowed...)
				}
				if mixed == "last" {
					answers = append(allowed, answers...)
				}
				proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
				proxy.resolver = fakeDNS{addresses: answers}
				got, err := proxy.resolvePinned(context.Background(), "example.com")
				if !errors.Is(err, core.ErrPolicyDenied) || len(got) != 0 {
					t.Fatalf("addresses=%v err=%v", got, err)
				}
			})
		}
	}
}

func TestHTTPPrivateDestinationsRemainAuthorizedAndPinned(t *testing.T) {
	for _, address := range []string{"10.0.0.8", "172.16.0.8", "192.168.0.8", "fd12::8"} {
		t.Run(address, func(t *testing.T) {
			var output bytes.Buffer
			logger, err := logging.New(logging.Config{Writer: &output, Format: logging.FormatJSON})
			if err != nil {
				t.Fatal(err)
			}
			authorizer := &fakeAuthorizer{}
			proxy := New(authorizer, fakeSources{environment: "env-a"})
			lookups := 0
			proxy.resolver = resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
				lookups++
				if host != "example.com" || lookups != 1 {
					return nil, errors.New("unexpected resolution")
				}
				// Deduplicate mapped IPv4 forms and allow a mixed public/private set.
				return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("93.184.216.34").To4()}, {IP: net.ParseIP(address)}}, nil
			})
			var dials []string
			served := make(chan error, 1)
			proxy.dial = func(_ context.Context, network, target string) (net.Conn, error) {
				dials = append(dials, target)
				if network != "tcp" {
					return nil, errors.New("unexpected network")
				}
				if target == "93.184.216.34:80" {
					return nil, errors.New("first address unavailable")
				}
				if target != net.JoinHostPort(address, "80") {
					return nil, errors.New("unpinned target")
				}
				conn, peer := net.Pipe()
				go func() {
					defer func() { _ = peer.Close() }()
					_ = peer.SetDeadline(time.Now().Add(3 * time.Second))
					req, err := http.ReadRequest(bufio.NewReader(peer))
					if err == nil && req.Host != "example.com" {
						err = errors.New("hostname changed")
					}
					if err == nil {
						_, err = io.WriteString(peer, "HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n")
					}
					served <- err
				}()
				return conn, nil
			}
			req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
			req = req.WithContext(logging.WithLogger(req.Context(), logger))
			req.RemoteAddr = "10.200.0.20:1234"
			out := httptest.NewRecorder()
			proxy.ServeHTTP(out, req)
			if out.Code != http.StatusNoContent {
				t.Fatalf("status=%d body=%s", out.Code, out.Body)
			}
			if err := <-served; err != nil {
				t.Fatal(err)
			}
			if output.Len() != 0 {
				t.Fatalf("successful fallback logged as failure: %s", output.String())
			}
			if lookups != 1 || !reflect.DeepEqual(dials, []string{"93.184.216.34:80", net.JoinHostPort(address, "80")}) {
				t.Fatalf("lookups=%d dials=%v", lookups, dials)
			}
			if len(authorizer.requests) != 1 || authorizer.requests[0].Host != "example.com" {
				t.Fatal(authorizer.requests)
			}
			authorizer.err = core.ErrPolicyDenied
			out = httptest.NewRecorder()
			proxy.ServeHTTP(out, req)
			if out.Code != http.StatusForbidden || lookups != 1 || len(dials) != 2 {
				t.Fatal("denial reused previous private-address grant")
			}
		})
	}
}

func assertUpstreamLog(t *testing.T, output string, reason, protocol string, port int) {
	t.Helper()
	var record map[string]any
	decoder := json.NewDecoder(strings.NewReader(output))
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("log: %v: %s", err, output)
	}
	for key, want := range map[string]any{"level": "ERROR", "component": "proxy", "operation": "egress_connect", "environment_id": "env-a", "target_host": "example.com", "target_port": float64(port), "protocol": protocol, "reason": reason} {
		if record[key] != want {
			t.Fatalf("%s=%v want %v: %s", key, record[key], want, output)
		}
	}
	if decoder.Decode(&record) != io.EOF {
		t.Fatalf("duplicate or malformed log: %s", output)
	}
	if strings.Contains(output, "PRIVATE") {
		t.Fatalf("raw private data reached log: %s", output)
	}
}

func TestUpstreamFailureClassification(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodConnect} {
		for _, tc := range []struct {
			name   string
			dns    fakeDNS
			reason string
			dial   bool
		}{
			{"lookup", fakeDNS{err: errors.New("PRIVATE resolver output")}, "dns_lookup_failed", false},
			{"empty", fakeDNS{}, "dns_empty_result", false},
			{"loopback", fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}, "address_loopback", false},
			{"mixed", fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("10.0.0.8")}, {IP: net.ParseIP("::1")}}}, "address_loopback", false},
			{"link-local", fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}}, "address_disallowed", false},
			{"malformed", fakeDNS{addresses: []net.IPAddr{{IP: net.IP{1, 2, 3}}}}, "address_disallowed", false},
			{"zone", fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("fd12::1"), Zone: "PRIVATE"}}}, "address_disallowed", false},
			{"canceled", fakeDNS{err: context.Canceled}, "canceled", false},
			{"timeout", fakeDNS{err: context.DeadlineExceeded}, "timeout", false},
			{"dial", fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("10.0.0.8")}}}, "dial_failed", true},
		} {
			// Real CONNECT/SNI dial failures are tested separately below.
			if method == http.MethodConnect && tc.dial {
				continue
			}
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				var output bytes.Buffer
				logger, err := logging.New(logging.Config{Writer: &output, Format: logging.FormatJSON})
				if err != nil {
					t.Fatal(err)
				}
				proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
				proxy.resolver = tc.dns
				dials := 0
				proxy.dial = func(context.Context, string, string) (net.Conn, error) {
					dials++
					return nil, errors.New("PRIVATE dial output")
				}
				req := httptest.NewRequest(method, "http://example.com/PRIVATE?secret=PRIVATE", nil)
				req.RemoteAddr = "10.200.0.20:1234"
				req.Header.Set("Authorization", "Bearer PRIVATE")
				req.Header.Set("Cookie", "PRIVATE")
				req.Header.Set("Proxy-Authorization", "PRIVATE")
				protocol, port := "http", 80
				if method == http.MethodConnect {
					req.Host = "example.com:443"
					protocol, port = "https", 443
				}
				req = req.WithContext(logging.WithLogger(req.Context(), logger))
				out := httptest.NewRecorder()
				proxy.ServeHTTP(out, req)
				if out.Code != http.StatusBadGateway || strings.Contains(out.Body.String(), "PRIVATE") {
					t.Fatalf("response=%v", out)
				}
				if (tc.dial && dials != 1) || (!tc.dial && dials != 0) {
					t.Fatalf("dials=%d", dials)
				}
				assertUpstreamLog(t, output.String(), tc.reason, protocol, port)
			})
		}
	}
}

func TestHTTPResponseFailureLog(t *testing.T) {
	var output bytes.Buffer
	logger, err := logging.New(logging.Config{Writer: &output, Format: logging.FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
	proxy.resolver = fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("10.0.0.8")}}}
	proxy.dial = func(context.Context, string, string) (net.Conn, error) {
		conn, peer := net.Pipe()
		go func() {
			defer func() { _ = peer.Close() }()
			_ = peer.SetDeadline(time.Now().Add(3 * time.Second))
			if _, err := http.ReadRequest(bufio.NewReader(peer)); err == nil {
				_, _ = io.WriteString(peer, "PRIVATE invalid HTTP response\r\n\r\n")
			}
		}()
		return conn, nil
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "10.200.0.20:1234"
	req = req.WithContext(logging.WithLogger(req.Context(), logger))
	out := httptest.NewRecorder()
	proxy.ServeHTTP(out, req)
	if out.Code != http.StatusBadGateway || strings.Contains(out.Body.String(), "PRIVATE") {
		t.Fatalf("response=%v", out)
	}
	assertUpstreamLog(t, output.String(), "upstream_request_failed", "http", 80)
}

type upstreamLogWriter chan string

func (w upstreamLogWriter) Write(data []byte) (int, error) {
	w <- string(data)
	return len(data), nil
}

func TestCONNECTDialFailureLogAfterSNI(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	output := make(upstreamLogWriter, 1)
	logger, err := logging.New(logging.Config{Writer: output, Format: logging.FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(logging.WithLogger(context.Background(), logger))
	defer cancel()
	proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
	proxy.resolver = fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("192.168.1.8")}}}
	proxy.dial = func(_ context.Context, network, target string) (net.Conn, error) {
		if network != "tcp" || target != "192.168.1.8:443" {
			return nil, errors.New("PRIVATE unpinned target")
		}
		return nil, errors.New("PRIVATE dial error")
	}
	done := make(chan error, 1)
	go func() { done <- proxy.Serve(ctx, managedPeerListener{listener}) }()
	defer func() { cancel(); <-done }()
	client, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = io.WriteString(client, "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: http.MethodConnect})
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("CONNECT=%v %v", response, err)
	}
	if _, err := client.Write(clientHello(t)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := client.Read(one[:]); err != io.EOF {
		t.Fatalf("expected closed failed tunnel: %v", err)
	}
	select {
	case entry := <-output:
		assertUpstreamLog(t, entry, "dial_failed", "https", 443)
	case <-time.After(time.Second):
		t.Fatal("missing CONNECT failure log")
	}
}
