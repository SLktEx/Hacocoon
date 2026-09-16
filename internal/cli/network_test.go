package cli

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	networkrelay "github.com/SLktEx/Hacocoon/internal/network"
	capability "github.com/SLktEx/Hacocoon/internal/policy"
)

type networkCLIClient struct {
	snapshot                          capability.PolicySnapshot
	edits                             []capability.PolicySnapshot
	sessions                          []networkrelay.Session
	revoked                           []string
	rules                             []networkrelay.RuleSpec
	reads                             int
	readErr, replaceErr, operationErr error
}

func (c *networkCLIClient) ReadConfiguration(context.Context) (capability.PolicySnapshot, error) {
	c.reads++
	return c.snapshot, c.readErr
}
func (c *networkCLIClient) ReplaceConfiguration(_ context.Context, edit capability.PolicySnapshot) (capability.PolicySnapshot, error) {
	c.edits = append(c.edits, edit)
	if c.replaceErr != nil {
		return capability.PolicySnapshot{}, c.replaceErr
	}
	if edit.Revision != c.snapshot.Revision {
		return capability.PolicySnapshot{}, core.ErrIncompatibleState
	}
	c.snapshot = edit
	return edit, nil
}
func (c *networkCLIClient) ListNetworkConnections(context.Context) ([]networkrelay.Session, error) {
	return c.sessions, c.operationErr
}
func (c *networkCLIClient) RevokeNetworkConnection(_ context.Context, id string) error {
	c.revoked = append(c.revoked, id)
	return c.operationErr
}
func (c *networkCLIClient) AddNetworkRule(_ context.Context, spec networkrelay.RuleSpec) (capability.PolicyRule, error) {
	c.rules = append(c.rules, spec)
	return capability.PolicyRule{Capability: "network.connect", Decision: spec.Decision, Environment: spec.Environment}, c.operationErr
}

func TestNetworkHostRegistrationPreservesPolicyAndUsesFreshIdentity(t *testing.T) {
	ctx := context.Background()
	policy := capability.PolicyFile{Default: core.PolicyDeny, Rules: []capability.PolicyRule{{Capability: "git", Action: "fetch", Resource: "trusted/repo", Decision: core.PolicyAllow}}, SavedDecisions: []capability.PolicyRule{{Capability: "network.connect", Environment: "dev", Decision: core.PolicyDeny}}}
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	c := &networkCLIClient{snapshot: capability.PolicySnapshot{Revision: "sha256:" + strings.Repeat("a", 64), Policy: data}}
	run := func(args ...string) string {
		t.Helper()
		var out, diagnostic strings.Builder
		if code := networkCommand(ctx, c, args, &out, &diagnostic); code != 0 {
			t.Fatal(args, code, diagnostic.String())
		}
		return out.String()
	}
	var instances []string
	for i := 0; i < 2; i++ {
		raw := run("host", "add", "--address", "127.0.0.1", "--port", "8080", "preview", "--json")
		var service capability.NetworkService
		if err := json.Unmarshal([]byte(raw), &service); err != nil || capability.ValidateNetworkService(service) != nil || service.Name != "preview" || service.Protocol != "tcp" || service.Address != "127.0.0.1" || service.Port != 8080 {
			t.Fatal(raw, err)
		}
		instances = append(instances, service.Instance)
		var persisted capability.PolicyFile
		if err := json.Unmarshal(c.snapshot.Policy, &persisted); err != nil {
			t.Fatal(err)
		}
		if persisted.Default != policy.Default || !reflect.DeepEqual(persisted.Rules, policy.Rules) || !reflect.DeepEqual(persisted.SavedDecisions, policy.SavedDecisions) || len(persisted.NetworkServices) != 1 || persisted.NetworkServices[0] != service {
			t.Fatal("host registration rewrote unrelated policy", persisted)
		}
		if c.edits[len(c.edits)-1].Revision != "sha256:"+strings.Repeat("a", 64) {
			t.Fatal("read revision was not retained")
		}
		var listed []capability.NetworkService
		if err := json.Unmarshal([]byte(run("host", "list", "--json")), &listed); err != nil || len(listed) != 1 || listed[0] != service {
			t.Fatal(listed, err)
		}
		var out, diagnostic strings.Builder
		before := len(c.edits)
		if networkCommand(ctx, c, []string{"host", "add", "--address", "127.0.0.1", "--port", "8081", "preview"}, &out, &diagnostic) != 1 || len(c.edits) != before {
			t.Fatal("duplicate registration silently rebound authority")
		}
		run("host", "remove", "preview")
		persisted = capability.PolicyFile{}
		if err := json.Unmarshal(c.snapshot.Policy, &persisted); err != nil || len(persisted.NetworkServices) != 0 {
			t.Fatal("removed service remains active", err, persisted)
		}
	}
	if instances[0] == instances[1] {
		t.Fatal("re-registration reused revoked identity")
	}
}

func TestNetworkCLIPropagatesRuleScopeLifetimeAndControllerFailures(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "en")
	ctx := context.Background()
	c := &networkCLIClient{sessions: []networkrelay.Session{{ID: "connection-a", State: "active"}}}
	var out, diagnostic strings.Builder
	if networkCommand(ctx, c, []string{"list", "--json"}, &out, &diagnostic) != 0 || !strings.Contains(out.String(), "connection-a") {
		t.Fatal(out.String(), diagnostic.String())
	}
	if networkCommand(ctx, c, []string{"revoke", "connection-a"}, io.Discard, &diagnostic) != 0 || !reflect.DeepEqual(c.revoked, []string{"connection-a"}) {
		t.Fatal(c.revoked)
	}
	for _, decision := range []string{"allow", "deny", "ask"} {
		before := time.Now().UTC()
		args := []string{"rule", "--env", "dev", "--kind", "external", "--target", "example.invalid", "--port", "443", "--duration", "37s", "--ttl", "2m", "--scope", "environment", "--decision", decision, "--json"}
		if code := networkCommand(ctx, c, args, io.Discard, &diagnostic); code != 0 {
			t.Fatal(code, diagnostic.String())
		}
		r := c.rules[len(c.rules)-1]
		want := core.PolicyDecision(decision)
		if decision == "ask" {
			want = core.PolicyRequireApproval
		}
		if r.Environment != "dev" || r.Scope != "environment" || r.Decision != want || r.Connection.Target != "example.invalid" || r.Connection.Protocol != "tcp" || r.Connection.Port != 443 || r.Connection.DurationSeconds != 37 || r.ExpiresAt.Before(before.Add(2*time.Minute)) || r.ExpiresAt.After(time.Now().Add(2*time.Minute)) {
			t.Fatal("rule authority/lifetime changed", r)
		}
	}
	c.operationErr = core.ErrCapabilityStale
	for _, args := range [][]string{{"list"}, {"revoke", "old"}, {"rule", "--target", "example.invalid", "--port", "443", "--env", "dev", "--decision", "allow"}} {
		out.Reset()
		diagnostic.Reset()
		if code := networkCommand(ctx, c, args, &out, &diagnostic); code != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), core.ErrCapabilityStale.Error()) {
			t.Fatal("controller refusal reported as success", code, out.String(), diagnostic.String())
		}
	}
}

func TestNetworkCLIRefusesInvalidArgumentsAndUncertainPolicyEdits(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"list", "extra"}, {"list", "--json", "--json"}, {"host", "add"}, {"host", "add", "--unknown"}, {"host", "add", "--address", "0.0.0.0", "--port", "80", "bad"}, {"host", "add", "--address", "127.0.0.1", "--port", "0", "bad"}, {"host", "list", "extra"}, {"rule", "--unknown"}, {"rule", "--duration", "500ms"}, {"rule", "--duration", "1500ms"}, {"rule", "--ttl", "0s"}, {"rule", "extra"}} {
		c := &networkCLIClient{}
		if code := networkCommand(context.Background(), c, args, io.Discard, io.Discard); code != 2 || c.reads != 0 || len(c.rules) != 0 || len(c.edits) != 0 {
			t.Fatal("invalid args reached policy mutation", args, code, c)
		}
	}
	for _, mode := range []string{"read", "malformed", "replace", "missing"} {
		verbs := []string{"add", "list", "remove"}
		if mode == "replace" {
			verbs = []string{"add", "remove"}
		}
		if mode == "missing" {
			verbs = []string{"remove"}
		}
		for _, verb := range verbs {
			t.Run(mode+"-"+verb, func(t *testing.T) {
				c := &networkCLIClient{snapshot: capability.PolicySnapshot{Revision: "sha256:" + strings.Repeat("a", 64), Policy: json.RawMessage(`{"default":"deny","rules":[],"network_services":[{"name":"preview","instance":"svc-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","address":"127.0.0.1","port":80,"protocol":"tcp"}]}`)}}
				args := []string{"host", verb}
				if verb == "add" {
					args = append(args, "--address", "127.0.0.1", "--port", "80", "new")
				}
				if verb == "remove" {
					args = append(args, "preview")
				}
				switch mode {
				case "read":
					c.readErr = core.ErrRuntimeUnavailable
				case "malformed":
					c.snapshot.Policy = []byte("{")
				case "replace":
					c.replaceErr = core.ErrIncompatibleState
				case "missing":
					args = []string{"host", "remove", "missing"}
				}
				var out, diagnostic strings.Builder
				if code := networkCommand(context.Background(), c, args, &out, &diagnostic); code != 1 || out.Len() != 0 || diagnostic.Len() == 0 {
					t.Fatal("failed edit reported success", code, out.String(), diagnostic.String())
				}
				if mode != "replace" && len(c.edits) != 0 {
					t.Fatal("unvalidated policy reached replacement")
				}
			})
		}
	}
}

func TestNetworkListenerValidatesLoopbackAndClosesOnCancellation(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "text")
	for _, protocol := range []string{"tcp", "udp"} {
		for _, listen := range []string{"0.0.0.0:0", "localhost:0", "192.0.2.1:80", "bad", "[::]:0"} {
			args := []string{protocol, "--target", "example.invalid", "--port", "443", "--listen", listen}
			if code := networkListenCommand(context.Background(), args, io.Discard, io.Discard); code != 2 {
				t.Fatal("unsafe listener accepted", args, code)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var out, diagnostic strings.Builder
		args := []string{protocol, "--target", "example.invalid", "--port", "443", "--duration", "1s", "--json"}
		if code := networkListenCommand(ctx, args, &out, &diagnostic); code != 0 {
			t.Fatal(code, diagnostic.String())
		}
		var result struct {
			Listen   string `json:"listen"`
			Protocol string `json:"protocol"`
			Duration int    `json:"duration_seconds"`
		}
		if err := json.Unmarshal([]byte(out.String()), &result); err != nil || result.Protocol != protocol || result.Duration != 1 {
			t.Fatal(out.String(), err)
		}
		if protocol == "tcp" {
			listener, err := net.Listen("tcp", result.Listen)
			if err != nil {
				t.Fatal("canceled listener retained socket", err)
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
		} else {
			address, err := net.ResolveUDPAddr("udp", result.Listen)
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.ListenUDP("udp", address)
			if err != nil {
				t.Fatal("canceled UDP socket retained", err)
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestNetworkListenerRefusesInvalidSetupAndOutputFailure(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "text")
	for _, args := range [][]string{{"tcp", "--json", "--json"}, {"tcp", "--unknown"}, {"udp", "extra"}, {"tcp", "--duration", "500ms"}, {"udp", "--target", "example.invalid", "--port", "0"}} {
		if code := networkListenCommand(context.Background(), args, io.Discard, io.Discard); code != 2 {
			t.Fatal(args, code)
		}
	}
	closed, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	c := &networkCLIClient{sessions: []networkrelay.Session{{ID: "active"}}}
	if code := networkCommand(context.Background(), c, []string{"list", "--json"}, closed, io.Discard); code != 1 {
		t.Fatal("output failure hidden", code)
	}
	for _, protocol := range []string{"tcp", "udp"} {
		args := []string{protocol, "--target", "example.invalid", "--port", "443", "--duration", "1s"}
		if code := networkListenCommand(context.Background(), args, closed, io.Discard); code != 1 {
			t.Fatal("ready-address write failure ignored", protocol, code)
		}
		t.Setenv("HACO_LOG_FORMAT", "unsupported")
		if code := networkListenCommand(context.Background(), args, io.Discard, io.Discard); code != 2 {
			t.Fatal("invalid log format accepted", code)
		}
		t.Setenv("HACO_LOG_FORMAT", "text")
	}
}
