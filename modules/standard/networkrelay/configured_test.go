package networkrelay

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net/netip"
	"testing"
	"time"
)

type configFixture struct {
	policy   capability.PolicyFile
	replaces int
}

func (c *configFixture) Snapshot(context.Context) (capability.PolicySnapshot, error) {
	data, _ := json.Marshal(c.policy)
	return capability.PolicySnapshot{Revision: "fixed", Policy: data}, nil
}
func (c *configFixture) Replace(_ context.Context, s capability.PolicySnapshot) (capability.PolicySnapshot, error) {
	c.replaces++
	return s, json.Unmarshal(s.Policy, &c.policy)
}

type dnsFixture struct {
	addresses                   []netip.Addr
	err                         error
	environment, instance, host string
	calls                       int
}

func (d *dnsFixture) ResolveInstance(_ context.Context, env, instance, host string) ([]netip.Addr, error) {
	d.environment, d.instance, d.host = env, instance, host
	d.calls++
	return append([]netip.Addr(nil), d.addresses...), d.err
}
func TestResolutionPinsEvidenceAndKeepsDNSPermissionSeparate(t *testing.T) {
	dns := &dnsFixture{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.2")}}
	a := &testAuthority{instance: testInstance}
	targets := &ConfiguredTargets{Configuration: &configFixture{}, Authority: a, DNS: dns, ExternalAllowed: func(context.Context, netip.Addr) error { return nil }}
	spec := Spec{Kind: "external", Target: "development.example", Protocol: "tcp", Port: 5432, DurationSeconds: 30}
	original, err := targets.Resolve(context.Background(), Source{"source", testInstance}, spec)
	if err != nil || dns.environment != "source" || dns.instance != testInstance || dns.host != spec.Target {
		t.Fatal(original, dns, err)
	}
	dns.addresses = []netip.Addr{netip.MustParseAddr("192.0.2.3")}
	replacement, err := targets.Resolve(context.Background(), Source{"source", testInstance}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if original.Addresses[0] == replacement.Addresses[0] {
		t.Fatal("new DNS answer silently reused")
	}
	first, _ := requestFor(Source{"source", testInstance}, original, 30)
	second, _ := requestFor(Source{"source", testInstance}, replacement, 30)
	if first.Attributes["addresses"] == second.Attributes["addresses"] {
		t.Fatal("DNS rebind did not change connection authority")
	}
	if err := targets.Verify(context.Background(), original); err != nil || dns.calls != 2 {
		t.Fatal("existing peer implicitly resolved again", dns.calls, err)
	}
	dns.err = core.ErrPolicyDenied
	if _, err := targets.Resolve(context.Background(), Source{"source", testInstance}, spec); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal(err)
	}
	dns.err = core.ErrNotFound
	if _, err := targets.Resolve(context.Background(), Source{"source", testInstance}, spec); failureReason(err) != "dns_failed" {
		t.Fatal(err)
	}
}
func TestHostRegistrationCannotRetargetExistingAuthority(t *testing.T) {
	service := capability.NetworkService{Name: "database", Instance: "svc-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Protocol: "udp", Address: "127.0.0.1", Port: 5432}
	config := &configFixture{policy: capability.PolicyFile{NetworkServices: []capability.NetworkService{service}}}
	targets := &ConfiguredTargets{Configuration: config, HostAllowed: func(context.Context, netip.Addr) error { return nil }}
	target, err := targets.Resolve(context.Background(), Source{"source", testInstance}, Spec{Kind: "host", Target: "database", Protocol: "udp", DurationSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*capability.NetworkService){
		func(s *capability.NetworkService) { s.Instance = "svc-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" },
		func(s *capability.NetworkService) { s.Address = "192.0.2.2" },
		func(s *capability.NetworkService) { s.Protocol = "tcp" },
		func(s *capability.NetworkService) { s.Port = 5433 },
	} {
		config.policy.NetworkServices = []capability.NetworkService{service}
		mutate(&config.policy.NetworkServices[0])
		if err := targets.Verify(context.Background(), target); !errors.Is(err, core.ErrCapabilityStale) {
			t.Fatal(err)
		}
	}
	config.policy.NetworkServices = nil
	if err := targets.Verify(context.Background(), target); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
}
func TestRuleDefaultsDoNotExtendApprovalToReplacementOrBroadenTarget(t *testing.T) {
	s, spec, _ := fixture(t, "tcp", 5432)
	for _, scope := range []string{"instance", "environment", "global"} {
		config := &configFixture{}
		rule, err := s.AddRule(context.Background(), config, RuleSpec{Environment: "source", Connection: spec, Scope: scope, Decision: core.PolicyAllow, ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil || config.replaces != 1 {
			t.Fatal(rule, err)
		}
		if (scope == "instance") != (rule.EnvironmentInstance == testInstance) || (scope == "global") != (rule.Environment == "*") ||
			rule.Attributes["destination_instance"] != "svc-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
			rule.Attributes["addresses"] != "127.0.0.1" || rule.Attributes["duration_seconds"] != "10" || rule.ExpiresAt == nil {
			t.Fatal(rule)
		}
	}
	config := &configFixture{}
	if _, err := s.AddRule(context.Background(), config, RuleSpec{Environment: "source", Connection: spec, Scope: "instance", Decision: core.PolicyAllow, ExpiresAt: time.Now().Add(-time.Second)}); err == nil || config.replaces != 0 {
		t.Fatal(err)
	}
}
func TestPolicyRevisionChangesAtExpiryWithoutWritingFile(t *testing.T) {
	expired := time.Now().Add(-time.Second)
	config := &configFixture{policy: capability.PolicyFile{Rules: []capability.PolicyRule{{ExpiresAt: &expired}}}}
	authority := &ConfiguredAuthority{Configuration: config}
	before, err := authority.PolicyRevision(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	config.policy.Rules[0].ExpiresAt = &later
	after, err := authority.PolicyRevision(context.Background())
	if err != nil || before == after {
		t.Fatal("rule activity missing from revision", err)
	}
}
