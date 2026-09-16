package networkrelay

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	capability "github.com/SLktEx/Hacocoon/internal/policy"
)

func TestNetworkRequestBindsCanonicalDestinationAndRefusesForgedAuthority(t *testing.T) {
	source := Source{Environment: "source", Instance: testInstance}
	target := Target{Kind: "external", Name: "example.invalid", Protocol: "tcp", Port: 443, Addresses: []netip.Addr{netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("192.0.2.2")}}
	request, err := requestFor(source, target, 31)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"destination_kind": "external", "destination_instance": "", "protocol": "tcp", "port": "443", "addresses": "192.0.2.2,2001:db8::1", "duration_seconds": "31"}
	if request.Environment != source.Environment || request.EnvironmentInstance != source.Instance || request.Resource != target.Name || request.Capability != Capability || request.Action != Action || !maps.Equal(request.Attributes, want) {
		t.Fatal("connection authority lost identity or canonical address set", request)
	}
	if (Provider{}).Capability() != Capability {
		t.Fatal("provider registered under wrong capability")
	}
	if result, err := (Provider{}).Execute(context.Background(), request); err != nil || result.Provider != "standard.network.authorization" {
		t.Fatal(result, err)
	}
	for _, mode := range []string{"capability", "action", "extra", "parameters", "port", "duration", "address", "address-order", "noncanonical-port", "owner", "source-instance", "source-name", "loopback", "duplicate-address", "mapped-address"} {
		t.Run(mode, func(t *testing.T) {
			bad := request
			bad.Attributes = maps.Clone(request.Attributes)
			switch mode {
			case "capability":
				bad.Capability = "host.exec"
			case "action":
				bad.Action = "execute"
			case "extra":
				bad.Attributes["command"] = "sh"
			case "parameters":
				bad.Parameters = map[string]string{"command": "sh"}
			case "port":
				bad.Attributes["port"] = "-x"
			case "duration":
				bad.Attributes["duration_seconds"] = "forever"
			case "address":
				bad.Attributes["addresses"] = "name.invalid"
			case "address-order":
				bad.Attributes["addresses"] = "2001:db8::1,192.0.2.2"
			case "noncanonical-port":
				bad.Attributes["port"] = "0443"
			case "owner":
				bad.Attributes["destination_instance"] = testInstance
			case "source-instance":
				bad.EnvironmentInstance = "name-only"
			case "source-name":
				bad.Environment = "-option"
			case "loopback":
				bad.Attributes["addresses"] = "127.0.0.1"
			case "duplicate-address":
				bad.Attributes["addresses"] = "192.0.2.2,192.0.2.2"
			case "mapped-address":
				bad.Attributes["addresses"] = "::ffff:192.0.2.2"
			}
			if result, err := (Provider{}).Execute(context.Background(), bad); err == nil || result.Provider != "" {
				t.Fatal("forged authority accepted", mode, result, err)
			}
		})
	}
}

func TestNetworkSpecificationRefusesAmbiguousDestinationsAndUnboundedLifetimes(t *testing.T) {
	valid := Spec{Kind: "external", Target: "example.invalid", Protocol: "tcp", Port: 443, DurationSeconds: 10}
	for _, mode := range []string{"zero-time", "long-time", "long-udp", "protocol", "empty-target", "long-target", "space", "path", "zone", "port", "host-port", "env-port", "name", "kind", "loopback", "link-local", "multicast", "unspecified", "noncanonical-host"} {
		t.Run(mode, func(t *testing.T) {
			spec := valid
			switch mode {
			case "zero-time":
				spec.DurationSeconds = 0
			case "long-time":
				spec.DurationSeconds = MaxDuration + 1
			case "long-udp":
				spec.Protocol = "udp"
				spec.DurationSeconds = MaxUDPDurations + 1
			case "protocol":
				spec.Protocol = "raw"
			case "empty-target":
				spec.Target = ""
			case "long-target":
				spec.Target = strings.Repeat("a", 254)
			case "space":
				spec.Target = " example.invalid"
			case "path":
				spec.Target = "example.invalid/admin"
			case "zone":
				spec.Target = "fe80::1%lo"
			case "port":
				spec.Port = 65536
			case "host-port":
				spec.Kind = "host"
				spec.Target = "database"
				spec.Port = -1
			case "env-port":
				spec.Kind = "environment"
				spec.Target = "database"
				spec.Port = 0
			case "name":
				spec.Kind = "environment"
				spec.Target = "bad.name"
			case "kind":
				spec.Kind = "management"
			case "loopback":
				spec.Target = "127.0.0.1"
			case "link-local":
				spec.Target = "169.254.254.1"
			case "multicast":
				spec.Target = "224.0.0.1"
			case "unspecified":
				spec.Target = "0.0.0.0"
			case "noncanonical-host":
				spec.Target = "EXAMPLE.invalid"
			}
			want := ValidateSpec(spec)
			if want == nil {
				t.Fatal("invalid target/lifetime accepted", spec)
			}
			if conn, session, err := OpenGuest(context.Background(), spec); !errors.Is(err, want) || conn != nil || session.ID != "" {
				t.Fatal("invalid request reached guest connection", session, err)
			}
		})
	}
	for _, spec := range []Spec{valid, {Kind: "host", Target: "database", Protocol: "udp", DurationSeconds: MaxUDPDurations}, {Kind: "environment", Target: "peer", Protocol: "tcp", Port: 22, DurationSeconds: MaxDuration}, {Kind: "external", Target: "2001:db8::1", Protocol: "tcp", Port: 443, DurationSeconds: 1}} {
		if err := ValidateSpec(spec); err != nil {
			t.Fatal("supported target refused", spec, err)
		}
	}
}

type authorityCatalog struct {
	env      core.Environment
	instance string
	failure  error
	calls    int
}

func (c *authorityCatalog) GetEnvironment(context.Context, string) (core.Environment, error) {
	return c.env, c.failure
}
func (c *authorityCatalog) EnvironmentInstance(_ context.Context, e core.Environment) (string, error) {
	c.calls++
	if !reflect.DeepEqual(e, c.env) {
		return "", core.ErrCapabilityStale
	}
	return c.instance, nil
}

func TestConfiguredAuthorityUsesCatalogIdentityAndPreservesPolicyAuditErrors(t *testing.T) {
	catalog := &authorityCatalog{env: core.Environment{Name: "dev", RuntimeRef: "persisted-owner"}, instance: testInstance}
	policy := &testAuthority{decision: core.PolicyDeny, auditError: core.ErrRuntimeUnavailable}
	a := &ConfiguredAuthority{Catalog: catalog, Policy: policy, Audit: policy}
	if got, err := a.CurrentInstance(context.Background(), "dev"); err != nil || got != testInstance || catalog.calls != 1 {
		t.Fatal(got, err)
	}
	catalog.failure = core.ErrNotFound
	if _, err := a.CurrentInstance(context.Background(), "missing"); !errors.Is(err, core.ErrNotFound) || catalog.calls != 1 {
		t.Fatal("missing environment acquired identity", err)
	}
	if result, err := a.Evaluate(context.Background(), core.CapabilityRequest{}); err != nil || result.Decision != core.PolicyDeny {
		t.Fatal(result, err)
	}
	event := core.CapabilityAuditEvent{Type: "connection-opened", RequestID: "request"}
	if err := a.Record(context.Background(), event); !errors.Is(err, policy.auditError) || len(policy.events) != 1 || !reflect.DeepEqual(policy.events[0], event) {
		t.Fatal("audit failure or event lost", err)
	}
}

type snapshotResult struct {
	view capability.PolicySnapshot
	err  error
}

func (s snapshotResult) Snapshot(context.Context) (capability.PolicySnapshot, error) {
	return s.view, s.err
}

func TestConfiguredTargetsFailClosedForUnknownOwnersAndPrivilegedEndpoints(t *testing.T) {
	ctx := context.Background()
	source := Source{"source", testInstance}
	service := capability.NetworkService{Name: "database", Instance: "svc-" + strings.Repeat("a", 32), Protocol: "tcp", Port: 5432, Address: "127.0.0.1"}
	config := &configFixture{policy: capability.PolicyFile{NetworkServices: []capability.NetworkService{service}}}
	targets := &ConfiguredTargets{Configuration: config, HostAllowed: func(context.Context, netip.Addr) error { return nil }}
	for _, port := range []int{18080, 8443, 2375, 2376} {
		config.policy.NetworkServices[0].Port = port
		if _, err := targets.Resolve(ctx, source, Spec{Kind: "host", Target: "database", Protocol: "tcp"}); !errors.Is(err, core.ErrPolicyDenied) {
			t.Fatal("management port exposed", port, err)
		}
	}
	config.policy.NetworkServices[0] = service
	for _, spec := range []Spec{{Kind: "host", Target: "database", Protocol: "udp"}, {Kind: "host", Target: "database", Protocol: "tcp", Port: 1234}, {Kind: "unknown"}} {
		if _, err := targets.Resolve(ctx, source, spec); err == nil {
			t.Fatal("wrong service contract accepted", spec)
		}
	}
	targets.HostAllowed = nil
	if _, err := targets.Resolve(ctx, source, Spec{Kind: "host", Target: "database", Protocol: "tcp"}); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal("missing host guard accepted", err)
	}
	targets.HostAllowed = func(context.Context, netip.Addr) error { return core.ErrRuntimeUnavailable }
	if _, err := targets.Resolve(ctx, source, Spec{Kind: "host", Target: "database", Protocol: "tcp"}); !errors.Is(err, core.ErrRuntimeUnavailable) {
		t.Fatal("host observation failure ignored", err)
	}
	for _, snapshot := range []snapshotResult{{err: core.ErrRuntimeUnavailable}, {view: capability.PolicySnapshot{Policy: []byte("{")}}} {
		targets.Configuration = snapshot
		if _, err := targets.Resolve(ctx, source, Spec{Kind: "host", Target: "database", Protocol: "tcp"}); err == nil {
			t.Fatal("unknown policy accepted")
		}
		if _, err := (&ConfiguredAuthority{Configuration: snapshot}).PolicyRevision(ctx); err == nil {
			t.Fatal("unknown policy acquired revision")
		}
	}
	config.policy.NetworkServices[0].Address = "0.0.0.0"
	targets.Configuration = config
	if _, err := targets.Resolve(ctx, source, Spec{Kind: "host", Target: "database", Protocol: "tcp"}); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal("invalid saved registration accepted", err)
	}
	targets.Authority = &ConfiguredAuthority{Catalog: &authorityCatalog{failure: core.ErrNotFound}}
	if _, err := targets.Resolve(ctx, source, Spec{Kind: "environment", Target: "missing", Protocol: "tcp", Port: 22}); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	if err := targets.Verify(ctx, Target{Kind: "environment", Name: "missing", Owner: testInstance}); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	a := &testAuthority{instance: testInstance}
	targets.Authority = a
	owned, err := targets.Resolve(ctx, source, Spec{Kind: "environment", Target: "peer", Protocol: "tcp", Port: 22})
	if err != nil || owned.Owner != testInstance || len(owned.Addresses) != 1 || owned.Addresses[0].String() != "127.0.0.1" {
		t.Fatal("provider namespace target lost identity", owned, err)
	}
	a.instance = "env-" + strings.Repeat("b", 32)
	if err := targets.Verify(ctx, owned); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("replacement inherited connection", err)
	}
	if err := targets.Verify(ctx, Target{Kind: "unknown"}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	targets.ExternalAllowed = nil
	if _, err := targets.Resolve(ctx, source, Spec{Kind: "external", Target: "192.0.2.2", Protocol: "tcp", Port: 443}); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal("external address bypassed guard", err)
	}
	targets.ExternalAllowed = func(context.Context, netip.Addr) error { return core.ErrPolicyDenied }
	if _, err := targets.Resolve(ctx, source, Spec{Kind: "external", Target: "192.0.2.2", Protocol: "tcp", Port: 443}); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal(err)
	}
}
