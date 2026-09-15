package agenthostcli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestAgentSSHAliasIsStableAndDoesNotExposeSessionID(t *testing.T) {
	sessionID := "copilot:/sensitive-session-name"
	first := agentSSHAlias(sessionID)
	second := agentSSHAlias(sessionID)
	if first != second {
		t.Fatalf("alias is not stable: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "haco-agent-") {
		t.Fatalf("unexpected alias: %q", first)
	}
	if strings.Contains(first, sessionID) || strings.Contains(first, "sensitive") {
		t.Fatalf("alias exposes raw session identity: %q", first)
	}
}

func TestEnsureSSHIncludeIsIdempotent(t *testing.T) {
	home := t.TempDir()
	if err := ensureSSHInclude(home); err != nil {
		t.Fatal(err)
	}
	if err := ensureSSHInclude(home); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		t.Fatal(err)
	}
	const include = "Include ~/.ssh/hacocoon/*.conf"
	if strings.Count(string(content), include) != 1 {
		t.Fatalf("include count = %d, content=%q", strings.Count(string(content), include), string(content))
	}
}

func TestManagedSSHConfigRoundTrip(t *testing.T) {
	home := t.TempDir()
	alias := agentSSHAlias("session-a")
	path := managedConfigPath(home, alias)
	want := managedSSHConfig{
		Alias:        alias,
		Connection:   testAgentConnection(),
		IdentityFile: "~/.ssh/id test",
	}
	if err := writeManagedSSHConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readManagedSSHConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch: got=%+v want=%+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("managed config permissions = %o", info.Mode().Perm())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"ProxyCommand /usr/local/bin/haco stream", "StrictHostKeyChecking yes", "User root", "IdentitiesOnly yes"} {
		if !strings.Contains(string(content), required) {
			t.Fatalf("managed config missing %q: %s", required, content)
		}
	}
}

func TestManagedSSHConfigRejectsInjectionValues(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "managed.conf")
	cases := []managedSSHConfig{
		{Alias: "haco-agent-good\nHost-evil", Connection: testAgentConnection(), IdentityFile: "~/.ssh/id_ed25519"},
		{Alias: "haco-agent-good", Connection: testAgentConnection(), IdentityFile: "~/.ssh/id_ed25519\nProxyCommand evil"},
		{Alias: "haco agent bad", Connection: testAgentConnection(), IdentityFile: "~/.ssh/id_ed25519"},
	}
	for _, config := range cases {
		if err := writeManagedSSHConfig(path, config); err == nil {
			t.Fatalf("unsafe managed SSH config unexpectedly accepted: %+v", config)
		}
	}
}

func TestReusableSSHConnectionRequiresAliasIdentityAndExactTarget(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	connections := []core.ClientConnection{
		{ID: "tcp-2222", Kind: "tcp", Port: 2222},
		testAgentConnection(),
	}
	previous := managedSSHConfig{Alias: "haco-agent-abcd", Connection: testAgentConnection(), IdentityFile: "~/.ssh/id_ed25519"}

	if got := reusableSSHConnection(previous, previous.Alias, previous.IdentityFile, connections); got.ID != previous.Connection.ID {
		t.Fatal("same grant not reused")
	}
	if got := reusableSSHConnection(previous, "haco-agent-other", previous.IdentityFile, connections); got.ID != "" {
		t.Fatalf("different alias must not reuse old connection: %+v", got)
	}
	if got := reusableSSHConnection(previous, previous.Alias, "~/.ssh/other", connections); got.ID != "" {
		t.Fatalf("different identity must not reuse old connection: %+v", got)
	}
	previous.Connection.Target.Instance = "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if got := reusableSSHConnection(previous, previous.Alias, previous.IdentityFile, connections); got.ID != "" {
		t.Fatalf("different generation must not reuse old connection: %+v", got)
	}
}

func TestFindSSHConnectionIgnoresNonSSHConnections(t *testing.T) {
	connections := []core.ClientConnection{
		{ID: "tcp-2222", Kind: "tcp", Port: 2222},
		testAgentConnection(),
	}
	if got := findSSHConnection(connections, "tcp-2222"); got.ID != "" {
		t.Fatalf("non-SSH connection must not match: %+v", got)
	}
	if got := findSSHConnection(connections, "ssh-one"); got.ID != "ssh-one" {
		t.Fatalf("expected SSH connection, got %+v", got)
	}
}

func testAgentConnection() core.ClientConnection {
	return core.ClientConnection{ID: "ssh-one", Kind: "ssh", User: "root", TargetPort: 22, HostPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f", Target: &core.StreamTarget{Environment: "agent-demo", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"}}
}
