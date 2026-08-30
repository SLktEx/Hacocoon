package egress

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeResolver struct {
	addresses map[string][]netip.Addr
}

func (r fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	return append([]netip.Addr(nil), r.addresses[host]...), nil
}

type captureRequester struct {
	requests []core.CapabilityRequest
	output   string
	err      error
}

func (r *captureRequester) Request(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	r.requests = append(r.requests, req)
	return core.CapabilityResult{Output: r.output}, r.err
}

func egressRequest(environment, protocol, hostname string, port int) core.CapabilityRequest {
	return core.CapabilityRequest{
		Capability: Capability,
		Action: connectAction,
		Resource: targetResource(protocol, hostname, port),
		Environment: environment,
		Attributes: map[string]string{
			"hostname": hostname,
			"protocol": protocol,
			"port":     string(rune('0' + port/10)) + string(rune('0' + port%10)),
		},
	}
}

func TestProviderPinsSortedPublicAddress(t *testing.T) {
	provider := newProviderWithResolver(fakeResolver{addresses: map[string][]netip.Addr{
		"api.example.com.": {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")},
	}})
	req := core.CapabilityRequest{
		Capability: Capability,
		Action: connectAction,
		Resource: "https://api.example.com:443",
		Environment: "env-a",
		Attributes: map[string]string{"hostname": "api.example.com", "protocol": "https", "port": "443"},
	}
	result, err := provider.Execute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "1.1.1.1:443" {
		t.Fatalf("output = %q", result.Output)
	}
}

func TestProviderRejectsPrivateOrMixedDNSAnswers(t *testing.T) {
	for name, addresses := range map[string][]netip.Addr{
		"private": {netip.MustParseAddr("10.0.0.7")},
		"mixed":   {netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("169.254.169.254")},
		"nat64":   {netip.MustParseAddr("64:ff9b::a00:7")},
	} {
		t.Run(name, func(t *testing.T) {
			provider := newProviderWithResolver(fakeResolver{addresses: map[string][]netip.Addr{"api.example.com.": addresses}})
			req := core.CapabilityRequest{
				Capability: Capability, Action: connectAction, Resource: "https://api.example.com:443", Environment: "env-a",
				Attributes: map[string]string{"hostname": "api.example.com", "protocol": "https", "port": "443"},
			}
			_, err := provider.Execute(context.Background(), req)
			if !errors.Is(err, core.ErrPolicyDenied) {
				t.Fatalf("error = %v, want ErrPolicyDenied", err)
			}
		})
	}
}

func TestCanonicalHostnameRejectsDirectIPAndAmbiguousNames(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "1.1.1.1", "localhost", "example.com@evil.test", "éxample.com"} {
		if _, err := canonicalHostname(value); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("canonicalHostname(%q) error = %v", value, err)
		}
	}
}

func TestAuthorizationScopesSameIPByHostnameAndEnvironment(t *testing.T) {
	requester := &captureRequester{output: "1.1.1.1:443"}
	service := New(requester, t.TempDir())
	for _, tc := range []struct{ env, host string }{
		{"env-a", "one.example.com"},
		{"env-a", "two.example.com"},
		{"env-b", "one.example.com"},
	} {
		if _, err := service.authorize(context.Background(), tc.env, "https", tc.host, 443); err != nil {
			t.Fatal(err)
		}
	}
	if len(requester.requests) != 3 {
		t.Fatalf("requests = %d", len(requester.requests))
	}
	if requester.requests[0].Resource == requester.requests[1].Resource {
		t.Fatalf("different hostnames shared policy resource: %#v", requester.requests)
	}
	if requester.requests[0].Environment == requester.requests[2].Environment {
		t.Fatalf("different environments shared policy scope: %#v", requester.requests)
	}
}

func TestAuthorizationRejectsUnsafeProviderOutput(t *testing.T) {
	requester := &captureRequester{output: "127.0.0.1:443"}
	service := New(requester, t.TempDir())
	_, err := service.authorize(context.Background(), "env-a", "https", "api.example.com", 443)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
}

func TestHTTPTargetRequiresAbsoluteURLAndMatchingHost(t *testing.T) {
	u, _ := url.Parse("http://api.example.com/path")
	host, port, err := httpTarget(u, "api.example.com")
	if err != nil || host != "api.example.com" || port != 80 {
		t.Fatalf("target = %s:%d err=%v", host, port, err)
	}
	if _, _, err := httpTarget(u, "other.example.com"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("Host mismatch error = %v", err)
	}
	direct, _ := url.Parse("http://127.0.0.1/")
	if _, _, err := httpTarget(direct, "127.0.0.1"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("direct IP error = %v", err)
	}
}

func TestStableSocketNameChangesAcrossEnvironmentGeneration(t *testing.T) {
	created := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	base := core.Environment{
		Name: "demo", RuntimeRef: "haco-runtime-v1:abc", CreatedAt: created,
		Workspace: core.Workspace{ID: "workspace-1"},
	}
	first, err := stableSocketName(base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := stableSocketName(base)
	if err != nil || first != second {
		t.Fatalf("restart identity changed: first=%q second=%q err=%v", first, second, err)
	}
	base.CreatedAt = created.Add(time.Nanosecond)
	third, err := stableSocketName(base)
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("recreated Environment reused old broker socket identity")
	}
}
