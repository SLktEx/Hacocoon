package dnsproxy

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	capability "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/nameresolution"
)

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
			service, err := capability.New(policy{decision}, nil, sink, Provider{Resolver: resolver})
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
	service, err := capability.New(policy{core.PolicyAllow}, nil, &audit{failure: true}, Provider{Resolver: lookup})
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
