//go:build linux

package vscodecli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
)

func TestHelpSucceedsWithoutPreparingWorkspace(t *testing.T) {
	state, _, _ := editorCommandFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		for _, command := range []string{"", "open", "delete"} {
			for _, flag := range []string{"--help", "-h"} {
				args := []string{flag}
				if command != "" {
					args = []string{command, flag}
				}
				var out, diagnostic bytes.Buffer
				if err := runAdapterWithIO(ctx, args, &out, &diagnostic, language); err != nil {
					t.Fatalf("%s %v: %v", language, args, err)
				}
				key := "vscode.help"
				if command != "" {
					key = "vscode." + command
				}
				if diagnostic.Len() != 0 || !strings.Contains(out.String(), language.Text(key)) {
					t.Fatalf("%s %v: stdout=%q stderr=%q", language, args, out.String(), diagnostic.String())
				}
			}
		}
	}
	state.Lock()
	defer state.Unlock()
	if len(state.created) != 0 || len(state.deleted) != 0 {
		t.Fatal("help mutated an Environment")
	}
}

func TestInvalidFlagKeepsHelpOnDiagnosticStream(t *testing.T) {
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		var out, diagnostic bytes.Buffer
		err := runAdapterWithIO(context.Background(), []string{"open", "--unknown"}, &out, &diagnostic, language)
		if err == nil || out.Len() != 0 || !strings.Contains(diagnostic.String(), language.Text("vscode.open")) {
			t.Fatalf("%s: error=%v stdout=%q stderr=%q", language, err, out.String(), diagnostic.String())
		}
	}
}
