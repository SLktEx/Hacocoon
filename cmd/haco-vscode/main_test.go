//go:build linux

package main

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
}
