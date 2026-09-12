package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func TestEveryCommandHelpIsLocalAndUsesStdout(t *testing.T) {
	// Invalid transport ensures help cannot require a running controller or create
	// resources. Each public metadata path must be handled before normal dispatch.
	t.Setenv("HACO_CONTROLLER_SOCKET", "/missing/roadmap-help.sock")
	for _, language := range []string{"C", "ja_JP.UTF-8"} {
		setCLITestLocale(t, language)
		for _, page := range helpPages {
			for _, flag := range []string{"--help", "-h"} {
				args := append(strings.Fields(page.path), flag)
				code, out, diagnostic := captureRun(t, args...)
				if code != 0 || diagnostic != "" || !strings.Contains(out, "haco "+page.path) || !strings.Contains(out, page.example) {
					t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, out, diagnostic)
				}
			}
		}
	}
}

func TestHelpDoesNotInterceptExecutionArgumentsOrUnknownCommands(t *testing.T) {
	for _, args := range [][]string{{"run", "--", "echo", "--help"}, {"env", "missing", "--help"}, {"env", "delete", "name", "--help"}, {"_git-agent", "--help"}} {
		var out bytes.Buffer
		if requestedCommandHelp(args, &out) || out.Len() != 0 {
			t.Fatalf("unexpected interception: %v", args)
		}
	}
	code, out, diagnostic := captureRun(t, "env", "missing", "--help")
	if code != 2 || out != "" || !strings.Contains(diagnostic, "haco env <command>") {
		t.Fatal(code, out, diagnostic)
	}
}

func TestHelpMetadataIsUniqueAndKeepsNewDevelopmentCommands(t *testing.T) {
	seen := map[string]bool{}
	for _, page := range helpPages {
		if seen[page.path] || cliui.English.Text(page.message) == page.message || cliui.Japanese.Text(page.message) == page.message {
			t.Fatalf("invalid page %+v", page)
		}
		seen[page.path] = true
	}
	for _, path := range []string{"env forward", "network tcp", "network udp", "workspace prepare", "workspace fork"} {
		if !seen[path] {
			t.Fatal(path)
		}
	}
	for _, path := range []string{"env switch-base", "plugin oci seed", "_git-agent"} {
		if seen[path] {
			t.Fatal("legacy/internal command restored", path)
		}
	}
}
