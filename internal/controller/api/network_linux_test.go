//go:build linux

package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/network"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

const networkAPIInstance = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type networkAPICatalog struct{}

func (networkAPICatalog) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	if name != "dev" {
		return core.Environment{}, core.ErrNotFound
	}
	return core.Environment{Name: name, RuntimeRef: "incus:owned"}, nil
}
func (networkAPICatalog) EnvironmentInstance(_ context.Context, env core.Environment) (string, error) {
	if env.Name != "dev" || env.RuntimeRef != "incus:owned" {
		return "", core.ErrCapabilityStale
	}
	return networkAPIInstance, nil
}

func TestNetworkManagementRoundTripPersistsExactRuleAndRevokesOnlyNamedConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	root := t.TempDir()
	evaluator := capability.NewFilePolicyEvaluator(filepath.Join(root, "policy.json"))
	audit := capability.NewJSONLAudit(filepath.Join(root, "audit", "events.jsonl"))
	configuration := &capability.PolicyConfiguration{Evaluator: evaluator, Audit: audit}
	permissions, err := capability.New(evaluator, nil, audit, networkrelay.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	permissions.ConfigureEnvironmentIdentity(networkAPICatalog{})
	authority := &networkrelay.ConfiguredAuthority{Catalog: networkAPICatalog{}, Configuration: configuration, Policy: evaluator, Audit: audit}
	service := &networkrelay.Service{Authority: authority, Capabilities: permissions, Targets: &networkrelay.ConfiguredTargets{Configuration: configuration, Authority: authority, ExternalAllowed: func(_ context.Context, a netip.Addr) error {
		if a.String() != "192.0.2.2" {
			return core.ErrPolicyDenied
		}
		return nil
	}}}
	t.Cleanup(service.Close)
	peers := make(chan net.Conn, 2)
	service.Dial = func(_ context.Context, protocol, address string) (net.Conn, error) {
		if protocol != "tcp" || address != "192.0.2.2:443" {
			return nil, core.ErrPolicyDenied
		}
		client, peer := net.Pipe()
		peers <- peer
		return client, nil
	}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterNetwork(s, service); err != nil {
			t.Fatal(err)
		}
		if err := RegisterNetworkRules(s, service, configuration); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	if initial, err := client.ListNetworkConnections(ctx); err != nil || len(initial) != 0 {
		t.Fatal(initial, err)
	}
	connection := networkrelay.Spec{Kind: "external", Target: "192.0.2.2", Protocol: "tcp", Port: 443, DurationSeconds: 10}
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	rule, err := client.AddNetworkRule(ctx, networkrelay.RuleSpec{Environment: "dev", Connection: connection, Decision: core.PolicyAllow, Scope: "instance", ExpiresAt: expires})
	if err != nil {
		t.Fatal(err)
	}
	wantAttributes := map[string]string{"destination_kind": "external", "destination_instance": "", "protocol": "tcp", "port": "443", "addresses": "192.0.2.2", "duration_seconds": "10"}
	if rule.Environment != "dev" || rule.EnvironmentInstance != networkAPIInstance || rule.Resource != "192.0.2.2" || rule.Decision != core.PolicyAllow || rule.ExpiresAt == nil || !rule.ExpiresAt.Equal(expires) || !reflect.DeepEqual(rule.Attributes, wantAttributes) {
		t.Fatal("wire rule lost exact authority", rule)
	}
	snapshot, err := configuration.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var saved capability.PolicyFile
	if err := json.Unmarshal(snapshot.Policy, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Rules) != 1 || !reflect.DeepEqual(saved.Rules[0], rule) {
		t.Fatal("rule receipt differs from persisted policy", saved.Rules, rule)
	}
	first, err := service.Open(ctx, networkrelay.Source{Environment: "dev", Instance: networkAPIInstance}, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := service.Open(ctx, networkrelay.Source{Environment: "dev", Instance: networkAPIInstance}, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	firstPeer, secondPeer := <-peers, <-peers
	defer func() { _ = firstPeer.Close(); _ = secondPeer.Close() }()
	listed, err := client.ListNetworkConnections(ctx)
	if err != nil || len(listed) != 2 {
		t.Fatal(listed, err)
	}
	for _, view := range listed {
		if view.State != "active" || view.Source.Instance != networkAPIInstance || view.Target.Port != 443 || view.RequestID == "" {
			t.Fatal("management view lost live identity", view)
		}
	}
	if err := firstPeer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := client.RevokeNetworkConnection(ctx, first.Session.ID); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if n, err := firstPeer.Read(b[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("revoked socket remained live", n, err)
	}
	if err := second.Validate(); err != nil {
		t.Fatal("revoke invalidated another connection", err)
	}
	listed, err = client.ListNetworkConnections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]networkrelay.Session{}
	for _, view := range listed {
		states[view.ID] = view
	}
	if states[first.Session.ID].Reason != "revoked" || states[first.Session.ID].State != "closed" || states[second.Session.ID].State != "active" {
		t.Fatal("revocation result lost", states)
	}
	for _, id := range []string{"bad", strings.Repeat("b", 32)} {
		err := client.RevokeNetworkConnection(ctx, id)
		want := "invalid_argument"
		if len(id) == 32 {
			want = "not_found"
		}
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != want {
			t.Fatal("revocation status changed", id, err)
		}
	}
	for _, method := range []string{MethodNetworkRevoke, MethodNetworkRule} {
		var status *control.StatusError
		if err := client.wire.Call(ctx, method, "not an object", nil); !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatal("malformed request accepted", method, err)
		}
	}
	if _, err := client.AddNetworkRule(ctx, networkrelay.RuleSpec{}); err == nil {
		t.Fatal("invalid rule accepted")
	}
	unchanged, err := configuration.Snapshot(ctx)
	if err != nil || unchanged.Revision != snapshot.Revision {
		t.Fatal("invalid requests changed policy", err)
	}
}

func TestNetworkRegistrationRejectsMissingServiceAndDuplicateMethods(t *testing.T) {
	service := &networkrelay.Service{}
	if err := RegisterNetwork(nil, service); !errors.Is(err, control.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if err := RegisterNetwork(control.NewServer(), nil); !errors.Is(err, control.ErrInvalidArgument) {
		t.Fatal(err)
	}
	s := control.NewServer()
	if err := RegisterNetwork(s, service); err != nil {
		t.Fatal(err)
	}
	if err := RegisterNetwork(s, service); err == nil {
		t.Fatal("duplicate handler replaced live network management")
	}
}
