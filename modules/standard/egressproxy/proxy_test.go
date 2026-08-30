package egressproxy

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeAuthorizer struct {
	requests []core.EgressRequest
	err      error
}

func (f *fakeAuthorizer) Authorize(_ context.Context, request core.EgressRequest) (core.EgressGrant, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return core.EgressGrant{}, f.err
	}
	return core.EgressGrant{Environment: request.Environment, Host: request.Host, Port: request.Port, Protocol: request.Protocol}, nil
}

type fakeSources struct {
	environment string
	err         error
}

func (f fakeSources) ResolveEnvironment(_ context.Context, _ net.IP) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.environment, nil
}

type fakeDNS struct {
	addresses []net.IPAddr
	err       error
}

func (f fakeDNS) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	return f.addresses, f.err
}

func TestParseAuthorityRejectsDirectIP(t *testing.T) {
	for _, authority := range []string{"1.1.1.1:443", "127.0.0.1:443", "[::1]:443"} {
		if _, _, err := parseAuthority(authority, 443); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("parseAuthority(%q) error = %v, want ErrInvalidArgument", authority, err)
		}
	}
}

func TestResolvePinnedRejectsPrivateOrMixedDNSAnswers(t *testing.T) {
	proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
	proxy.resolver = fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("127.0.0.1")}}}
	if _, err := proxy.resolvePinned(context.Background(), "example.com"); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("mixed DNS error = %v, want ErrPolicyDenied", err)
	}

	proxy.resolver = fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("10.0.0.8")}}}
	if _, err := proxy.resolvePinned(context.Background(), "example.com"); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("private DNS error = %v, want ErrPolicyDenied", err)
	}
}

func TestResolvePinnedAcceptsPublicDNSAnswerWithoutRedialResolution(t *testing.T) {
	proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
	proxy.resolver = fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
	addresses, err := proxy.resolvePinned(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 1 || addresses[0].String() != "93.184.216.34" {
		t.Fatalf("addresses = %#v", addresses)
	}
}

func TestReadClientHelloServerName(t *testing.T) {
	clientSide, proxySide := net.Pipe()
	defer proxySide.Close()
	client := tls.Client(clientSide, &tls.Config{ServerName: "Allowed.Example", MinVersion: tls.VersionTLS12})
	done := make(chan struct{})
	go func() {
		_ = client.SetDeadline(time.Now().Add(2 * time.Second))
		_ = client.Handshake()
		_ = client.Close()
		close(done)
	}()

	prefix, serverName, err := readClientHelloServerName(proxySide)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefix) == 0 || serverName != "Allowed.Example" {
		t.Fatalf("prefix=%d serverName=%q", len(prefix), serverName)
	}
	_ = proxySide.Close()
	<-done
}

func TestHTTPAuthorityMismatchFailsBeforeAuthorization(t *testing.T) {
	authorizer := &fakeAuthorizer{}
	proxy := New(authorizer, fakeSources{environment: "env-a"})
	request := httptest.NewRequest(http.MethodGet, "http://allowed.example/path", nil)
	request.RemoteAddr = "10.200.0.20:40000"
	request.Host = "denied.example"
	recorder := httptest.NewRecorder()

	proxy.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if len(authorizer.requests) != 0 {
		t.Fatalf("authorization happened for mismatched authority: %#v", authorizer.requests)
	}
}

func TestHTTPDirectIPFailsBeforeAuthorization(t *testing.T) {
	authorizer := &fakeAuthorizer{}
	proxy := New(authorizer, fakeSources{environment: "env-a"})
	request := httptest.NewRequest(http.MethodGet, "http://1.1.1.1/path", nil)
	request.RemoteAddr = "10.200.0.20:40000"
	recorder := httptest.NewRecorder()

	proxy.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if len(authorizer.requests) != 0 {
		t.Fatalf("authorization happened for IP literal: %#v", authorizer.requests)
	}
}
