package egressproxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type authorizerFunc func(context.Context, core.EgressRequest) (core.EgressGrant, error)

func (f authorizerFunc) Authorize(ctx context.Context, r core.EgressRequest) (core.EgressGrant, error) {
	return f(ctx, r)
}

type resolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f resolverFunc) LookupIPAddr(ctx context.Context, h string) ([]net.IPAddr, error) {
	return f(ctx, h)
}

func TestTransportsRejectMismatchedGrantBeforeDNS(t *testing.T) {
	changes := map[string]func(*core.EgressGrant){
		"environment": func(g *core.EgressGrant) { g.Environment = "other" },
		"hostname":    func(g *core.EgressGrant) { g.Host = "other.example" },
		"port":        func(g *core.EgressGrant) { g.Port++ },
		"protocol":    func(g *core.EgressGrant) { g.Protocol = "tcp" },
		"empty":       func(g *core.EgressGrant) { *g = core.EgressGrant{} },
	}
	for _, method := range []string{http.MethodGet, http.MethodConnect} {
		for name, change := range changes {
			t.Run(method+"/"+name, func(t *testing.T) {
				proxy := New(authorizerFunc(func(_ context.Context, r core.EgressRequest) (core.EgressGrant, error) {
					g := core.EgressGrant{Environment: r.Environment, Host: r.Host, Port: r.Port, Protocol: r.Protocol}
					change(&g)
					return g, nil
				}), fakeSources{environment: "env-a"})
				lookups, dials := 0, 0
				proxy.resolver = resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
					lookups++
					return nil, errors.New("unexpected lookup")
				})
				proxy.dial = func(context.Context, string, string) (net.Conn, error) {
					dials++
					return nil, errors.New("unexpected dial")
				}
				req := httptest.NewRequest(method, "http://example.com/path", nil)
				req.RemoteAddr = "10.200.0.20:1234"
				if method == http.MethodConnect {
					req.Host = "example.com:443"
				}
				out := httptest.NewRecorder()
				proxy.ServeHTTP(out, req)
				if out.Code != http.StatusForbidden || lookups != 0 || dials != 0 {
					t.Fatalf("status=%d DNS=%d dial=%d", out.Code, lookups, dials)
				}
			})
		}
	}
}

func TestPinnedResolutionRejectsLateCanceledResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
	proxy.resolver = resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
		cancel()
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	})
	addresses, err := proxy.resolvePinned(ctx, "example.com")
	if !errors.Is(err, context.Canceled) || len(addresses) != 0 {
		t.Fatalf("addresses=%v err=%v", addresses, err)
	}
}

func TestPinnedDialClosesLateCanceledConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	proxy.dial = func(context.Context, string, string) (net.Conn, error) { cancel(); return conn, nil }
	got, err := proxy.dialPinned(ctx, []netip.Addr{netip.MustParseAddr("93.184.216.34")}, 443)
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("late connection accepted: %v %v", got, err)
	}
	_ = peer.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := peer.Write([]byte("x")); !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("peer remains open: %v", err)
	}
}

func TestEachTransportAttemptRequiresFreshAuthorizationAndResolution(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodConnect} {
		t.Run(method, func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			proxy := New(authorizer, fakeSources{environment: "env-a"})
			lookups := 0
			proxy.resolver = resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
				lookups++
				return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
			})
			proxy.dial = func(context.Context, string, string) (net.Conn, error) {
				return nil, errors.New("fixture stops before upstream")
			}
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest(method, "http://example.com/", nil)
				req.RemoteAddr = "10.200.0.20:1234"
				if method == http.MethodConnect {
					req.Host = "example.com:443"
				}
				proxy.ServeHTTP(httptest.NewRecorder(), req)
			}
			if len(authorizer.requests) != 2 || lookups != 2 {
				t.Fatalf("authorization=%d resolution=%d", len(authorizer.requests), lookups)
			}
			authorizer.err = core.ErrPolicyDenied
			req := httptest.NewRequest(method, "http://example.com/", nil)
			req.RemoteAddr = "10.200.0.20:1234"
			if method == http.MethodConnect {
				req.Host = "example.com:443"
			}
			out := httptest.NewRecorder()
			proxy.ServeHTTP(out, req)
			if out.Code != http.StatusForbidden || lookups != 2 {
				t.Fatal("denied request resolved or reused earlier grant", out.Code, lookups)
			}
		})
	}
}
