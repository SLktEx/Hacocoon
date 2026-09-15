//go:build linux

package vscodecli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultEnvironmentNameIsStableAndValid(t *testing.T) {
	path := filepath.Join("/tmp", "My Project_with symbols!")
	name := defaultEnvironmentName(path)
	if name != defaultEnvironmentName(path) || len(name) > 57 || !strings.HasPrefix(name, "vscode-my-project-with-symbols-") {
		t.Fatal(name)
	}
	if name == defaultEnvironmentName("/another/My Project_with symbols!") {
		t.Fatal("path collision")
	}
	for _, path := range []string{"/tmp/日本語", "/tmp/" + strings.Repeat("long-project-", 12)} {
		name := defaultEnvironmentName(path)
		if len(name) > 57 || strings.ContainsAny(name, "_ /\\") || name != defaultEnvironmentName(filepath.Join(filepath.Dir(path), ".", filepath.Base(path))) {
			t.Fatalf("invalid or unstable default name %q", name)
		}
	}
	if name := defaultEnvironmentName("/tmp/日本語"); !strings.HasPrefix(name, "vscode-workspace-") {
		t.Fatal("empty sanitized basename has no fallback", name)
	}
}
