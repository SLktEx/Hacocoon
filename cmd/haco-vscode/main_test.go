package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
