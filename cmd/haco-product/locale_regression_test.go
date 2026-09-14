package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Existing output snapshots are English. Locale-specific tests explicitly set
// all three variables, rather than depending on the developer's shell locale.
func TestMain(m *testing.M) {
	if err := os.Unsetenv("HACO_UI_LANGUAGE"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("LC_ALL", "C"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

const (
	trustedHostNoticeEnglish  = "Entering trusted haco-host. Host authority is available here; use an Environment for ordinary development work."
	trustedHostNoticeJapanese = "信頼済みの haco-host に入ります。ここでは Host 権限を利用できます。通常の開発作業には Environment を使用してください。"
)

func setCLITestLocale(t *testing.T, locale string) {
	t.Helper()
	t.Setenv("HACO_UI_LANGUAGE", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", locale)
}

func TestPresentationOverridePreservesJSONAndFailureExit(t *testing.T) {
	var baseline string
	for _, language := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", language)
		code, output, diagnostic := captureRun(t, "version", "--json")
		if code != 0 || diagnostic != "" || !json.Valid([]byte(output)) {
			t.Fatalf("invalid version: %d %q %q", code, output, diagnostic)
		}
		if baseline == "" {
			baseline = output
		} else if baseline != output {
			t.Fatal("presentation changed JSON")
		}
		code, output, diagnostic = captureRun(t, "unknown-command")
		if code != 2 || output != "" || !strings.Contains(diagnostic, `"unknown-command"`) {
			t.Fatalf("presentation changed failure: %d %q %q", code, output, diagnostic)
		}
		if cliLanguage() != cliui.Language(language) {
			t.Fatal("presentation override was not applied")
		}
	}
}

func TestLocalePreservesVersionJSONAndExitCodes(t *testing.T) {
	var baseline string
	for _, locale := range []string{"en_US.UTF-8", "ja_JP.UTF-8"} {
		t.Run(locale, func(t *testing.T) {
			setCLITestLocale(t, locale)
			code, stdout, stderr := captureRun(t, "version", "--json")
			if code != 0 || stderr != "" || !json.Valid([]byte(stdout)) {
				t.Fatalf("code=%d out=%q err=%q", code, stdout, stderr)
			}
			if baseline == "" {
				baseline = stdout
			} else if stdout != baseline {
				t.Fatal("language changed JSON bytes")
			}
			code, stdout, stderr = captureRun(t, "unknown-command")
			if code != 2 || stdout != "" || !strings.Contains(stderr, `"unknown-command"`) {
				t.Fatalf("code=%d out=%q err=%q", code, stdout, stderr)
			}
			if locale == "ja_JP.UTF-8" && !strings.Contains(stderr, "利用できません") {
				t.Fatal(stderr)
			}
		})
	}
}

func TestLocaleEnvironmentAndDoctorHelp(t *testing.T) {
	setCLITestLocale(t, "ja_JP.UTF-8")
	var out, diagnostic bytes.Buffer
	if code := environmentCommand(context.Background(), []string{"--help"}, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "使い方:\n  haco env") || diagnostic.Len() != 0 {
		t.Fatalf("code=%d diagnostic=%q", code, diagnostic.String())
	}
	out.Reset()
	diagnostic.Reset()
	if code := doctor(context.Background(), []string{"--help"}, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "使い方: haco doctor") || diagnostic.Len() != 0 {
		t.Fatalf("code=%d out=%q diagnostic=%q", code, out.String(), diagnostic.String())
	}
}

type localeApprovalClient struct {
	pending   []core.ApprovalRequest
	decisions []capability.ApprovalDecision
}

func (client *localeApprovalClient) PendingApprovals(context.Context) ([]core.ApprovalRequest, error) {
	return client.pending, nil
}
func (client *localeApprovalClient) DecideApproval(_ context.Context, id string, decision capability.ApprovalDecision) (core.CapabilityResult, error) {
	client.decisions = append(client.decisions, decision)
	return core.CapabilityResult{RequestID: id, Output: "must-not-leak", Provider: "must-not-leak"}, nil
}

func TestLocaleApprovalJSONAndDecisionAreIdentical(t *testing.T) {
	request := core.ApprovalRequest{RequestID: "locale-request", CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target-日本語", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111"}}
	for _, args := range [][]string{{"--json"}, {"--list", "--json"}} {
		var baseline string
		for _, locale := range []string{"en_US.UTF-8", "ja_JP.UTF-8"} {
			t.Run(strings.Join(args, "")+locale, func(t *testing.T) {
				setCLITestLocale(t, locale)
				client := &localeApprovalClient{pending: []core.ApprovalRequest{request}}
				var out, diagnostic bytes.Buffer
				code := approvalCommand(context.Background(), client, args, strings.NewReader("y\n"), &out, &diagnostic)
				if code != 0 || !json.Valid(out.Bytes()) || strings.Contains(out.String(), "must-not-leak") {
					t.Fatalf("code=%d out=%q diagnostic=%q", code, out.String(), diagnostic.String())
				}
				if baseline == "" {
					baseline = out.String()
				} else if baseline != out.String() {
					t.Fatal("locale changed approval JSON")
				}
				if args[0] == "--json" {
					if len(client.decisions) != 1 || !client.decisions[0].Approved || client.decisions[0].Save != "" {
						t.Fatalf("decisions=%+v", client.decisions)
					}
					if locale == "ja_JP.UTF-8" && !strings.Contains(diagnostic.String(), "未入力は拒否") {
						t.Fatal(diagnostic.String())
					}
				} else if len(client.decisions) != 0 || diagnostic.Len() != 0 {
					t.Fatal("listing made a decision or emitted a prompt")
				}
			})
		}
	}
}

func TestLocaleEmptyApprovalJSONRemainsNull(t *testing.T) {
	setCLITestLocale(t, "ja_JP.UTF-8")
	var out, diagnostic bytes.Buffer
	if code := approvalCommand(context.Background(), &localeApprovalClient{}, []string{"--json"}, strings.NewReader(""), &out, &diagnostic); code != 0 || out.String() != "null\n" || diagnostic.Len() != 0 {
		t.Fatalf("code=%d out=%q diagnostic=%q", code, out.String(), diagnostic.String())
	}
}
