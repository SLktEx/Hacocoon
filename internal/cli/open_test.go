//go:build linux

package cli

import (
	"strings"
	"testing"
)

func TestOpenClientChoiceIsExplicitAndValidatedBeforeSetup(t *testing.T) {
	code, _, err := captureRun(t, "open", "--client", "unknown", "dev")
	if code != 2 || !strings.Contains(err, "--client vscode|ssh") {
		t.Fatalf("%d %s", code, err)
	}
	code, out, err := captureRun(t, "open", "--help")
	if code != 0 || err != "" || !strings.Contains(out, "--client vscode|ssh|none") {
		t.Fatalf("%d %s %s", code, out, err)
	}
	if !strings.Contains(out, "--new") || !strings.Contains(out, "--snapshot") || strings.Contains(out, "path discovery is not implemented") {
		t.Fatalf("help does not describe the integrated Workspace entry: %s", out)
	}
}

func TestOpenLanguageExplainsInvalidOptionsBeforeConnecting(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			t.Setenv("HACO_CONTROL_SOCKET", "/no-controller-for-invalid-options.sock")
			for _, tc := range []struct {
				args   []string
				en, ja string
			}{
				{[]string{"--json", "dev"}, "requires a directory", "ディレクトリ"},

				{[]string{"--close", "dev"}, "require --port", "--portを指定"},
				{[]string{"--no-browser", "./project"}, "require --port", "--portを指定"},
				{[]string{"--port", "0", "dev"}, "1..65535", "1〜65535"},
				{[]string{"--port", "65536", "./project"}, "1..65535", "1〜65535"},
				{[]string{"--port", "8080", "--client", "ssh", "dev"}, "--client vscode", "--client vscode"},
				{[]string{"--port", "8080", "--client", "none", "./project"}, "--client vscode", "--client vscode"},
			} {
				code, out, diagnostic := captureRun(t, append([]string{"open"}, tc.args...)...)
				if code != 2 || out != "" || diagnostic == "" || strings.Contains(diagnostic, "no-controller-for-invalid-options") {
					t.Fatalf("%v: code=%d out=%q diagnostic=%q", tc.args, code, out, diagnostic)
				}
			}
		})
	}
}
