//go:build linux

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/experimental"
)

func TestPendingApprovalLanguageNeverSubmitsDecision(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			in, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatal(err)
			}
			defer in.Close()
			client := &fakeApprovalClient{requests: []core.ApprovalRequest{testApprovalRequest("pending")}}
			var out, diagnostic bytes.Buffer
			code := approvalCommand(context.Background(), client, nil, in, &out, &diagnostic)
			if code != 2 || client.calls != 0 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "[waiting_approval]") || !strings.Contains(diagnostic.String(), cliMessage("approval.terminal_required")) {
				t.Fatalf("code=%d decisions=%d stdout=%q stderr=%q", code, client.calls, out.String(), diagnostic.String())
			}
		})
	}
}

func TestExperimentalLanguagePreservesJSON(t *testing.T) {
	var baseline string
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			store := experimental.Store{Path: filepath.Join(t.TempDir(), "managed", "config.yaml")}
			var out, diagnostic bytes.Buffer
			code := experimentalCommand(context.Background(), store, []string{"edit", "vscode", "--json", "-"}, strings.NewReader(`{"settings":{"editor.formatOnSave":true}}`), &out, &diagnostic, nil)
			if code != 0 || out.Len() != 0 || !strings.Contains(diagnostic.String(), cliMessage("experimental.saved")) {
				t.Fatalf("save: %d %s %s", code, &out, &diagnostic)
			}
			diagnostic.Reset()
			code = experimentalCommand(context.Background(), store, []string{"edit", "vscode", "--json"}, strings.NewReader(""), &out, &diagnostic, nil)
			if code != 0 || diagnostic.Len() != 0 {
				t.Fatalf("read: %d %s", code, &diagnostic)
			}
			if language == "en" {
				baseline = out.String()
			} else if out.String() != baseline {
				t.Fatalf("translated JSON: %q != %q", out.String(), baseline)
			}
		})
	}
}
