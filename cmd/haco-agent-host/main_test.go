package main

import (
	"os"
	"path/filepath"
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
		Port:         2222,
		IdentityFile: "~/.ssh/id test",
	}
	if err := writeManagedSSHConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readManagedSSHConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
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
	for _, required := range []string{"HostName 127.0.0.1", "User root", "IdentitiesOnly yes"} {
		if !strings.Contains(string(content), required) {
			t.Fatalf("managed config missing %q: %s", required, content)
		}
	}
}

func TestManagedSSHConfigRejectsInjectionValues(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "managed.conf")
	cases := []managedSSHConfig{
		{Alias: "haco-agent-good\nHost-evil", Port: 2222, IdentityFile: "~/.ssh/id_ed25519"},
		{Alias: "haco-agent-good", Port: 2222, IdentityFile: "~/.ssh/id_ed25519\nProxyCommand evil"},
		{Alias: "haco agent bad", Port: 2222, IdentityFile: "~/.ssh/id_ed25519"},
	}
	for _, config := range cases {
		if err := writeManagedSSHConfig(path, config); err == nil {
			t.Fatalf("unsafe managed SSH config unexpectedly accepted: %+v", config)
		}
	}
}

func TestReusableSSHConnectionRequiresAliasIdentityAndCompatiblePort(t *testing.T) {
	connections := []core.ClientConnection{
		{ID: "tcp-2222", Kind: "tcp", Port: 2222},
		{ID: "ssh-2222", Kind: "ssh", Port: 2222, User: "root"},
	}
	previous := managedSSHConfig{Alias: "haco-agent-abcd", Port: 2222, IdentityFile: "~/.ssh/id_ed25519"}

	for _, requestedPort := range []int{0, 2222} {
		got := reusableSSHConnection(previous, previous.Alias, previous.IdentityFile, requestedPort, connections)
		if got.ID != "ssh-2222" {
			t.Fatalf("expected reusable SSH connection for port %d, got %+v", requestedPort, got)
		}
	}
	if got := reusableSSHConnection(previous, "haco-agent-other", previous.IdentityFile, 0, connections); got.Port != 0 {
		t.Fatalf("different alias must not reuse old connection: %+v", got)
	}
	if got := reusableSSHConnection(previous, previous.Alias, "~/.ssh/other", 0, connections); got.Port != 0 {
		t.Fatalf("different identity must not reuse old connection: %+v", got)
	}
	if got := reusableSSHConnection(previous, previous.Alias, previous.IdentityFile, 3333, connections); got.Port != 0 {
		t.Fatalf("different explicit port must not reuse old connection: %+v", got)
	}
}

func TestFindSSHConnectionIgnoresNonSSHConnections(t *testing.T) {
	connections := []core.ClientConnection{
		{ID: "tcp-2222", Kind: "tcp", Port: 2222},
		{ID: "ssh-3333", Kind: "ssh", Port: 3333},
	}
	if got := findSSHConnection(connections, 2222); got.Port != 0 {
		t.Fatalf("non-SSH connection must not match: %+v", got)
	}
	if got := findSSHConnection(connections, 3333); got.ID != "ssh-3333" {
		t.Fatalf("expected SSH connection, got %+v", got)
	}
}

func TestFreeLoopbackPortReturnsValidPort(t *testing.T) {
	port, err := freeLoopbackPort()
	if err != nil {
		t.Fatal(err)
	}
	if port < 1 || port > 65535 {
		t.Fatalf("invalid port: %d", port)
	}
}
