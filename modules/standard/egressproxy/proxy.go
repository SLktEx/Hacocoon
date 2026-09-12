package egressproxy

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/dnsproxy"
)

const DefaultPort = 18080

type Authorizer interface {
	Authorize(context.Context, core.EgressRequest) (core.EgressGrant, error)
}

type SourceResolver interface {
	ResolveEnvironment(context.Context, net.IP) (string, error)
}

type DNSResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type Proxy struct {
	operations     http.Handler
	nameResolution http.Handler
	authorizer     Authorizer
	sources        SourceResolver
	resolver       DNSResolver
	dial           func(context.Context, string, string) (net.Conn, error)
}

func New(authorizer Authorizer, sources SourceResolver) *Proxy {
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return &Proxy{
		authorizer: authorizer,
		sources:    sources,
		resolver:   net.DefaultResolver,
		dial:       dialer.DialContext,
	}
}

// NewWithNameResolution shares only the guarded listener and persisted source
// mapping. DNS requests still pass through their own Capability policy and audit.
func NewWithNameResolution(authorizer Authorizer, sources SourceResolver, lookup dnsproxy.Lookup) *Proxy {
	p := New(authorizer, sources)
	p.nameResolution = dnsproxy.NewHandler(lookup, sources)
	return p
}

// NewWithOperations admits optional operation handlers only on origin-form
// reserved paths. Each handler owns source authentication and capability policy.
func NewWithOperations(authorizer Authorizer, sources SourceResolver, lookup dnsproxy.Lookup, operations http.Handler) *Proxy {
	p := NewWithNameResolution(authorizer, sources, lookup)
	p.operations = operations
	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p == nil || p.authorizer == nil || p.sources == nil || p.resolver == nil || p.dial == nil {
		http.Error(w, "egress proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	if p.operations != nil && r.URL != nil && !r.URL.IsAbs() && strings.HasPrefix(r.RequestURI, "/_haco/operations/") {
		p.operations.ServeHTTP(w, r)
		return
	}
	if p.nameResolution != nil && r.URL != nil && !r.URL.IsAbs() && r.RequestURI == dnsproxy.Path {
		p.nameResolution.ServeHTTP(w, r)
		return
	}
	environment, err := p.resolveSource(r.Context(), r.RemoteAddr)
	if err != nil {
		http.Error(w, "unmanaged egress source", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r, environment)
		return
	}
	p.handleHTTP(w, r, environment)
}

func (p *Proxy) resolveSource(ctx context.Context, remote string) (string, error) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return "", core.ErrInvalidArgument
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return "", core.ErrPolicyDenied
	}
	return p.sources.ResolveEnvironment(ctx, ip)
}
