package egressproxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/dnsproxy"
	"golang.org/x/net/dns/dnsmessage"
)

type lookupFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f lookupFunc) Resolve(ctx context.Context, env, host string) ([]netip.Addr, error) {
	return f(ctx, env, host)
}

func TestNameResolutionRouteDoesNotAuthorizeConnections(t *testing.T) {
	calls := 0
	auth := &fakeAuthorizer{err: core.ErrPolicyDenied}
	proxy := NewWithNameResolution(auth, fakeSources{environment: "dev"}, lookupFunc(func(_ context.Context, env, host string) ([]netip.Addr, error) {
		calls++
		if env != "dev" || host != "intranet.example" {
			t.Error("incorrect DNS authority")
		}
		return []netip.Addr{netip.MustParseAddr("10.20.30.40")}, nil
	}))
	name, err := dnsmessage.NewName("intranet.example.")
	if err != nil {
		t.Fatal(err)
	}
	query, err := (&dnsmessage.Message{Questions: []dnsmessage.Question{{Name: name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}}).Pack()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(proxy)
	defer server.Close()
	response, err := server.Client().Post(server.URL+dnsproxy.Path, dnsproxy.ContentType, bytes.NewReader(query))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	var answer dnsmessage.Message
	if response.StatusCode != 200 || answer.Unpack(data) != nil || len(answer.Answers) != 1 || calls != 1 || len(auth.requests) != 0 {
		t.Fatal("local DNS route failed or granted connection authority")
	}
	// An external URL with the same path remains an ordinary egress request.
	external := httptest.NewRequest(http.MethodPost, "http://intranet.example"+dnsproxy.Path, bytes.NewReader(query))
	external.RemoteAddr = "10.0.0.5:54321"
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, external)
	if recorder.Code != http.StatusForbidden || calls != 1 || len(auth.requests) != 1 {
		t.Fatal("external URL bypassed egress policy using DNS route")
	}
}
