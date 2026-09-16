package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/network"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

type networkLanguageClient struct {
	snapshot capability.PolicySnapshot
	calls    []string
	revoked  string
	rule     networkrelay.RuleSpec
	failure  error
}

func newNetworkLanguageClient() *networkLanguageClient {
	return &networkLanguageClient{snapshot: capability.PolicySnapshot{Revision: "original-revision", Policy: json.RawMessage(`{"default":"deny","rules":[{"capability":"git.push","action":"push","resource":"main","decision":"require-approval"}]}`)}}
}
func (c *networkLanguageClient) ReadConfiguration(context.Context) (capability.PolicySnapshot, error) {
	c.calls = append(c.calls, "read")
	return c.snapshot, c.failure
}
func (c *networkLanguageClient) ReplaceConfiguration(_ context.Context, value capability.PolicySnapshot) (capability.PolicySnapshot, error) {
	c.calls = append(c.calls, "replace")
	if c.failure != nil {
		return capability.PolicySnapshot{}, c.failure
	}
	if value.Revision != c.snapshot.Revision {
		return capability.PolicySnapshot{}, core.ErrIncompatibleState
	}
	c.snapshot = value
	return value, nil
}
func (c *networkLanguageClient) ListNetworkConnections(context.Context) ([]networkrelay.Session, error) {
	c.calls = append(c.calls, "list")
	return []networkrelay.Session{}, c.failure
}
func (c *networkLanguageClient) RevokeNetworkConnection(_ context.Context, id string) error {
	c.calls = append(c.calls, "revoke")
	c.revoked = id
	return c.failure
}
func (c *networkLanguageClient) AddNetworkRule(_ context.Context, rule networkrelay.RuleSpec) (capability.PolicyRule, error) {
	c.calls = append(c.calls, "rule")
	c.rule = rule
	return capability.PolicyRule{Capability: networkrelay.Capability, Action: networkrelay.Action, Resource: rule.Connection.Target, Environment: rule.Environment, Decision: rule.Decision}, c.failure
}

func TestNetworkHostLanguageKeepsRegistrationSeparateFromPermission(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			client := newNetworkLanguageClient()
			var out, diag bytes.Buffer
			if code := networkCommand(context.Background(), client, []string{"host", "add", "--address", "127.0.0.1", "--port", "15432", "database"}, &out, &diag); code != 0 {
				t.Fatal(code, diag.String())
			}
			phrase := "Registration alone does not grant access"
			if language == "ja" {
				phrase = "登録だけでは通信を許可しません"
			}
			if !strings.Contains(out.String(), phrase) || !reflect.DeepEqual(client.calls, []string{"read", "replace"}) {
				t.Fatal(out.String(), client.calls)
			}
			var policy capability.PolicyFile
			if err := json.Unmarshal(client.snapshot.Policy, &policy); err != nil {
				t.Fatal(err)
			}
			if policy.Default != core.PolicyDeny || len(policy.Rules) != 1 || policy.Rules[0].Decision != core.PolicyRequireApproval || len(policy.NetworkServices) != 1 || policy.NetworkServices[0].Name != "database" || policy.NetworkServices[0].Port != 15432 || policy.NetworkServices[0].Instance == "" {
				t.Fatal(policy)
			}
			before := bytes.Clone(client.snapshot.Policy)
			client.calls = nil
			out.Reset()
			diag.Reset()
			if code := networkCommand(context.Background(), client, []string{"host", "add", "--address", "127.0.0.1", "--port", "15433", "database"}, &out, &diag); code != 1 || out.Len() != 0 || !reflect.DeepEqual(client.calls, []string{"read"}) || !bytes.Equal(before, client.snapshot.Policy) {
				t.Fatal(code, out.String(), diag.String(), client.calls)
			}
			client.calls = nil
			out.Reset()
			diag.Reset()
			if code := networkCommand(context.Background(), client, []string{"host", "remove", "database"}, &out, &diag); code != 0 || !reflect.DeepEqual(client.calls, []string{"read", "replace"}) || !strings.Contains(out.String(), "haco network host list") {
				t.Fatal(code, out.String(), diag.String(), client.calls)
			}
			// Decode into a fresh value so omitted fields cannot retain the previous view.
			policy = capability.PolicyFile{}
			if err := json.Unmarshal(client.snapshot.Policy, &policy); err != nil || len(policy.NetworkServices) != 0 || len(policy.Rules) != 1 || policy.Rules[0].Decision != core.PolicyRequireApproval || policy.Default != core.PolicyDeny {
				t.Fatal(err, policy)
			}
		})
	}
}

func TestNetworkLanguagePreservesJSONAndExactRequests(t *testing.T) {
	commands := [][]string{{"list", "--json"}, {"host", "list", "--json"}, {"revoke", "--json", "connection-1"}, {"rule", "--json", "--env", "dev", "--decision", "ask", "--target", "example.com", "--port", "443", "--duration", "1m", "--ttl", "1h"}}
	english := make([]string, len(commands))
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			for i, args := range commands {
				client := newNetworkLanguageClient()
				var out, diag bytes.Buffer
				before := time.Now()
				if code := networkCommand(context.Background(), client, args, &out, &diag); code != 0 || diag.Len() != 0 || len(client.calls) != 1 {
					t.Fatal(code, out.String(), diag.String(), client.calls)
				}
				if language == "en" {
					english[i] = out.String()
				} else if out.String() != english[i] {
					t.Fatal("JSON changed with language", out.String(), english[i])
				}
				if args[0] == "revoke" && (client.revoked != "connection-1" || out.Len() != 0) {
					t.Fatal("revocation changed", client.revoked, out.String())
				}
				if args[0] == "rule" && (client.rule.Decision != core.PolicyRequireApproval || client.rule.Scope != "instance" || client.rule.Environment != "dev" || client.rule.Connection.Target != "example.com" || client.rule.Connection.Port != 443 || client.rule.Connection.DurationSeconds != 60 || client.rule.ExpiresAt.Before(before.Add(time.Hour)) || client.rule.ExpiresAt.After(time.Now().Add(time.Hour))) {
					t.Fatal("rule intent changed", client.rule)
				}
			}
		})
	}
}

type networkLanguageBrokenWriter struct{}

func (networkLanguageBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestNetworkJapaneseFailureAndEmptyGuidance(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	client := newNetworkLanguageClient()
	var out, diag bytes.Buffer
	if code := networkCommand(context.Background(), client, []string{"list"}, &out, &diag); code != 0 || !strings.Contains(out.String(), "接続中の通信はありません") {
		t.Fatal(code, out.String(), diag.String())
	}
	out.Reset()
	diag.Reset()
	if code := networkCommand(context.Background(), client, []string{"host", "list"}, &out, &diag); code != 0 || !strings.Contains(out.String(), "Hostサービスは未登録") {
		t.Fatal(code, out.String(), diag.String())
	}
	client.failure = errors.New("original backend diagnostic")
	out.Reset()
	diag.Reset()
	if code := networkCommand(context.Background(), client, []string{"revoke", "exact-id"}, &out, &diag); code != 1 || out.Len() != 0 || !strings.Contains(diag.String(), "通信の操作に失敗") || !strings.Contains(diag.String(), "original backend diagnostic") {
		t.Fatal(code, out.String(), diag.String())
	}
	client.failure = nil
	client.calls = nil
	if code := networkCommand(context.Background(), client, []string{"revoke", "exact-id"}, networkLanguageBrokenWriter{}, &diag); code != 1 || !reflect.DeepEqual(client.calls, []string{"revoke"}) {
		t.Fatal("output failure replayed operation", code, client.calls)
	}
}

func TestNetworkListenerLanguageAndCancellation(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	for _, protocol := range []string{"tcp", "udp"} {
		t.Run(protocol, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var out, diag bytes.Buffer
			if code := networkListenCommand(ctx, []string{protocol, "--target", "example.com", "--port", "443", "--duration", "1m"}, &out, &diag); code != 0 || !strings.Contains(out.String(), "接続口を用意しました") || !strings.Contains(out.String(), "Ctrl+C") {
				t.Fatal(code, out.String(), diag.String())
			}
			out.Reset()
			diag.Reset()
			if code := networkListenCommand(ctx, []string{protocol, "--json", "--target", "example.com", "--port", "443", "--duration", "1m"}, &out, &diag); code != 0 {
				t.Fatal(code, diag.String())
			}
			var value struct {
				Listen, Protocol, Target string
				Duration                 int `json:"duration_seconds"`
			}
			if err := json.Unmarshal(out.Bytes(), &value); err != nil || value.Protocol != protocol || value.Target != "example.com" || value.Duration != 60 {
				t.Fatal(err, out.String())
			}
			// Rebind the exact emitted endpoint: cancellation must close the owned listener.
			if protocol == "tcp" {
				listener, err := net.Listen("tcp", value.Listen)
				if err != nil {
					t.Fatal(err)
				}
				_ = listener.Close()
			} else {
				listener, err := net.ListenPacket("udp", value.Listen)
				if err != nil {
					t.Fatal(err)
				}
				_ = listener.Close()
			}
			out.Reset()
			diag.Reset()
			if code := networkListenCommand(ctx, []string{protocol, "--target", "example.com", "--port", "443", "--duration", "1m", "--listen", "0.0.0.0:0"}, &out, &diag); code != 2 || out.Len() != 0 || !strings.Contains(diag.String(), "loopback") {
				t.Fatal(code, out.String(), diag.String())
			}
		})
	}
}
