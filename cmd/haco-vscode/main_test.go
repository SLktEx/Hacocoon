package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestDefaultEnvironmentNameIsStableAndValid(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "tmp", "My Project_with symbols!")
	first := defaultEnvironmentName(workspace)
	second := defaultEnvironmentName(workspace)
	if first != second {
		t.Fatalf("name is not stable: %q != %q", first, second)
	}
	if len(first) > 57 {
		t.Fatalf("name too long: %d %q", len(first), first)
	}
	if !strings.HasPrefix(first, "vscode-my-project-with-symbols-") {
		t.Fatalf("unexpected name: %q", first)
	}
}

func TestAdapterEnvironmentNameRejectsManagedConfigTraversal(t *testing.T) {
	for _, name := range []string{"../../../../tmp/victim", "../victim", "demo/../../victim", "demo\nHost evil"} {
		if err := validateAdapterEnvironmentName(name); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("name %q: expected invalid argument, got %v", name, err)
		}
	}
	if err := validateAdapterEnvironmentName("vscode-demo-0123abcd"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
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
	if err := ensureSSHInclude(home); err != nil {
		t.Fatal(err)
	}
	path := managedConfigPath(home, "dev")
	want := managedSSHConfig{
		Alias:        "haco-vscode-dev",
		Port:         2222,
		IdentityFile: filepath.Join(home, "keys", "id test"),
	}
	if err := writeManagedSSHConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readManagedSSHConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Alias != want.Alias || got.Port != want.Port || !samePath(got.IdentityFile, want.IdentityFile) {
		t.Fatalf("round trip mismatch: got=%+v want=%+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("managed config permissions = %o", info.Mode().Perm())
	}
}

func TestReusableSSHConnectionRequiresSameIdentityAndCompatiblePort(t *testing.T) {
	connections := []core.ClientConnection{
		{ID: "tcp-8080-3000", Kind: "tcp", Port: 8080},
		{ID: "ssh-2222", Kind: "ssh", Port: 2222, User: "root"},
	}
	previous := managedSSHConfig{Alias: "haco-vscode-dev", Port: 2222, IdentityFile: "~/.ssh/id_ed25519"}

	for _, requestedPort := range []int{0, 2222} {
		got := reusableSSHConnection(previous, "~/.ssh/id_ed25519", requestedPort, connections)
		if got.ID != "ssh-2222" {
			t.Fatalf("expected reusable SSH connection for requested port %d, got %+v", requestedPort, got)
		}
	}

	if got := reusableSSHConnection(previous, "~/.ssh/other", 0, connections); got.Port != 0 {
		t.Fatalf("changed identity must not reuse old connection: %+v", got)
	}
	if got := reusableSSHConnection(previous, "~/.ssh/id_ed25519", 3333, connections); got.Port != 0 {
		t.Fatalf("explicit different port must not reuse old connection: %+v", got)
	}
}

func TestFindSSHConnectionIgnoresNonSSHConnections(t *testing.T) {
	connections := []core.ClientConnection{
		{ID: "tcp-2222-22", Kind: "tcp", Port: 2222},
		{ID: "ssh-3333", Kind: "ssh", Port: 3333},
	}
	if got := findSSHConnection(connections, 2222); got.Port != 0 {
		t.Fatalf("non-SSH connection must not match: %+v", got)
	}
	if got := findSSHConnection(connections, 3333); got.ID != "ssh-3333" {
		t.Fatalf("expected SSH connection, got %+v", got)
	}
}
