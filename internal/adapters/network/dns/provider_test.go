package dnsproxy

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/network/dns"
	capability "github.com/SLktEx/Hacocoon/internal/policy"
)

const dnsInstance = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type dnsCatalog struct{ environment core.Environment }

func (c *dnsCatalog) GetEnvironment(context.Context, string) (core.Environment, error) {
	return c.environment, nil
}
func (c *dnsCatalog) EnvironmentInstance(_ context.Context, e core.Environment) (string, error) {
	if !e.Equal(c.environment) {
		return "", core.ErrCapabilityStale
	}
	return dnsInstance, nil
}
func ordinaryDNSCatalog() *dnsCatalog {
	return &dnsCatalog{core.Environment{Name: "dev", RuntimeRef: "runtime:dev"}}
}

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type policy struct{ decision core.PolicyDecision }

func (p policy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: p.decision}, nil
}

type audit struct {
	events  []core.CapabilityAuditEvent
	failure bool
}

func (a *audit) Record(_ context.Context, e core.CapabilityAuditEvent) error {
	a.events = append(a.events, e)
	if a.failure {
		return errors.New("audit unavailable")
	}
	return nil
}
func TestDNSUsesPolicyBeforePlatformResolverAndDoesNotGrantConnection(t *testing.T) {
	for _, decision := range []core.PolicyDecision{core.PolicyAllow, core.PolicyDeny, core.PolicyRequireApproval} {
		t.Run(string(decision), func(t *testing.T) {
			calls := 0
			sink := &audit{}
			resolver := resolverFunc(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
				calls++
				if network != "ip" || host != "intranet.example" || len(sink.events) < 2 {
					t.Fatal("lookup before policy/audit")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("unbounded platform lookup")
				}
				return []netip.Addr{netip.MustParseAddr("10.20.30.40")}, nil
			})
			service, err := capability.New(policy{decision}, nil, sink, Provider{Resolver: resolver, Environments: ordinaryDNSCatalog()})
			if err != nil {
				t.Fatal(err)
			}
			addresses, err := nameresolution.New(service).Resolve(context.Background(), "dev", "Intranet.Example.")
			if decision == core.PolicyAllow {
				if err != nil || len(addresses) != 1 || addresses[0].String() != "10.20.30.40" {
					t.Fatalf("addresses=%v err=%v", addresses, err)
				}
			} else if err == nil || calls != 0 {
				t.Fatal("denied or headless approval performed lookup")
			}
			for _, e := range sink.events {
				if e.Capability != "network.resolve" || e.Action != "lookup" || e.Resource != "intranet.example" || e.Environment != "dev" {
					t.Fatalf("incorrect authority %v", e)
				}
			}
		})
	}
}
func TestDNSAuditAndMalformedInputFailBeforeLookup(t *testing.T) {
	lookup := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("unexpected lookup")
		return nil, nil
	})
	service, err := capability.New(policy{core.PolicyAllow}, nil, &audit{failure: true}, Provider{Resolver: lookup, Environments: ordinaryDNSCatalog()})
	if err != nil {
		t.Fatal(err)
	}
	broker := nameresolution.New(service)
	for _, host := range []string{"valid.example", "127.0.0.1", "-option", "bad/name", "bad" + string(rune(0))} {
		if _, err := broker.Resolve(context.Background(), "dev", host); err == nil {
			t.Fatal("unsafe or unaudited lookup accepted")
		}
	}
}

type backendResolverFunc func(context.Context, string, string, string) ([]netip.Addr, error)

func (f backendResolverFunc) ResolveEnvironmentName(ctx context.Context, ref, instance, name string) ([]netip.Addr, error) {
	return f(ctx, ref, instance, name)
}
func TestDNSModeSelectionAndCreationReplacement(t *testing.T) {
	for _, mode := range []core.DNSMode{"", core.DNSHost, core.DNSBackend, core.DNSDisabled, "unknown"} {
		t.Run(string(mode), func(t *testing.T) {
			catalog := ordinaryDNSCatalog()
			catalog.environment.DNSMode = mode
			hostCalls, backendCalls := 0, 0
			p := Provider{Environments: catalog, Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				hostCalls++
				return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
			}), Backend: backendResolverFunc(func(_ context.Context, ref, instance, name string) ([]netip.Addr, error) {
				backendCalls++
				if ref != "runtime:dev" || instance != dnsInstance || name != "example.test" {
					t.Fatal("wrong resolver target")
				}
				return []netip.Addr{netip.MustParseAddr("10.0.0.2")}, nil
			})}
			req := core.CapabilityRequest{Capability: nameresolution.Capability, Action: nameresolution.Action, Environment: "dev", EnvironmentInstance: dnsInstance, Resource: "example.test"}
			_, err := p.Execute(context.Background(), req)
			allowed := mode.Valid() && mode != core.DNSDisabled
			if (err == nil) != allowed {
				t.Fatalf("error=%v", err)
			}
			wantHost, wantBackend := 0, 0
			if mode.Effective() == core.DNSHost {
				wantHost = 1
			}
			if mode == core.DNSBackend {
				wantBackend = 1
			}
			if hostCalls != wantHost || backendCalls != wantBackend {
				t.Fatalf("host=%d backend=%d", hostCalls, backendCalls)
			}
			req.EnvironmentInstance = "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			if _, err = p.Execute(context.Background(), req); err == nil {
				t.Fatal("stale instance accepted")
			}
			if hostCalls != wantHost || backendCalls != wantBackend {
				t.Fatal("stale lookup reached upstream")
			}
		})
	}
	catalog := ordinaryDNSCatalog()
	p := Provider{Environments: catalog, Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		catalog.environment.RuntimeRef = "replacement"
		return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
	})}
	req := core.CapabilityRequest{Capability: nameresolution.Capability, Action: nameresolution.Action, Environment: "dev", EnvironmentInstance: dnsInstance, Resource: "example.test"}
	if _, err := p.Execute(context.Background(), req); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("replacement accepted: %v", err)
	}
}
func TestDNSMissingCatalogAndBackendFailClosed(t *testing.T) {
	req := core.CapabilityRequest{Capability: nameresolution.Capability, Action: nameresolution.Action, Environment: "dev", Resource: "example.test"}
	catalog := ordinaryDNSCatalog()
	catalog.environment.DNSMode = core.DNSBackend
	for _, p := range []Provider{{}, {Environments: catalog, Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("backend fell back to host")
		return nil, nil
	})}} {
		if _, err := p.Execute(context.Background(), req); err == nil {
			t.Fatal("missing dependency accepted")
		}
	}
}
