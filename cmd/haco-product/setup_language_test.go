package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/projectsetup"
)

func TestProductSetupLocalizedHostOutcomes(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			for _, status := range []string{"", "setup_failed", "customization_failed", "busy"} {
				t.Run(status, func(t *testing.T) {
					var failure error
					if status != "" {
						failure = control.NewStatusError(status, "PRIVATE-BACKEND")
					}
					t.Setenv("HACO_CONTROL_SOCKET", productSetupServer(t, failure))
					var out, diag bytes.Buffer
					code := setup(context.Background(), nil, &out, &diag)
					want := 1
					if status == "" {
						want = 0
					}
					combined := out.String() + diag.String()
					if code != want || strings.Contains(combined, "PRIVATE-BACKEND") || !strings.Contains(diag.String(), "[succeeded] controller_readiness") {
						t.Fatalf("code=%d output=%s", code, combined)
					}
					if status == "" {
						phrase := "Host resources prepared"
						if language == "ja" {
							phrase = "Hostの準備が完了しました"
						}
						if !strings.Contains(out.String(), phrase) || !strings.Contains(out.String(), "haco doctor") {
							t.Fatal(combined)
						}
					} else {
						phrase := "Setup completion is not confirmed"
						if language == "ja" {
							phrase = "セットアップの完了を確認できませんでした"
						}
						if !strings.Contains(diag.String(), phrase) || !strings.Contains(diag.String(), "journalctl -u haco-controller.service") || out.Len() != 0 {
							t.Fatal(combined)
						}
						if status == "setup_failed" && strings.Contains(diag.String(), "level=ERROR") {
							t.Fatal("duplicated controller-owned ERROR")
						}
					}
				})
			}
		})
	}
}

func TestProductSetupLocalizedProjectPreservesOutputAndOutcome(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			for _, outcome := range []string{"empty", "applied", "cleared", "failed"} {
				t.Run(outcome, func(t *testing.T) {
					calls := 0
					path := productSetupServiceServer(t, productSetupFixture{}, func(server *control.Server) {
						if err := server.Register(controlapi.MethodProjectSetup, func(_ context.Context, raw json.RawMessage) (any, error) {
							calls++
							var req controlapi.ProjectSetupRequest
							if err := json.Unmarshal(raw, &req); err != nil || req.Environment != "dev" || req.Update.Script != nil || req.Update.Clear || req.Update.Reapply || req.Update.ResultOnly {
								t.Error("request changed", string(raw))
							}
							return controlapi.ProjectSetupResponse{Failed: outcome == "failed", FailureCode: "internal", Result: projectsetup.Result{Applied: outcome == "applied", Cleared: outcome == "cleared", FailureStage: "execute", Execution: core.ExecutionResult{Stdout: "Script: Project setup completed.\n", Stderr: "Script: 診断\n", ExitCode: 23, StdoutTruncated: true}}}, nil
						}); err != nil {
							t.Fatal(err)
						}
					})
					t.Setenv("HACO_CONTROL_SOCKET", path)
					var out, diag bytes.Buffer
					code := setup(context.Background(), []string{"dev"}, &out, &diag)
					want := 0
					if outcome == "failed" {
						want = 1
					}
					if code != want || calls != 1 || !strings.HasPrefix(out.String(), "Script: Project setup completed.\n") || !strings.HasPrefix(diag.String(), "Script: 診断\n") {
						t.Fatal(code, calls, out.String(), diag.String())
					}
					phrases := map[string]string{"empty": "No saved project setup", "applied": "Project setup completed.", "cleared": "Saved project setup removed.", "failed": "haco: project setup failed."}
					if language == "ja" {
						phrases = map[string]string{"empty": "プロジェクトの保存手順はありません", "applied": "プロジェクトのセットアップが完了しました", "cleared": "プロジェクトの保存手順を解除しました", "failed": "プロジェクトのセットアップに失敗しました"}
					}
					message := strings.TrimPrefix(out.String(), "Script: Project setup completed.\n") + diag.String()
					if !strings.Contains(message, phrases[outcome]) {
						t.Fatal(message)
					}
				})
			}
		})
	}
}

func TestProductSetupJapaneseHelpWithoutController(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	t.Setenv("HACO_CONTROL_SOCKET", "")
	var out, diag bytes.Buffer
	if code := setup(context.Background(), []string{"--help"}, &out, &diag); code != 0 || !strings.Contains(out.String(), "--script-result") || !strings.Contains(out.String(), "再実行") || strings.Contains(diag.String(), "controller_readiness") {
		t.Fatal(code, out.String(), diag.String())
	}
}
