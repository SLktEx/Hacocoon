package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/client/reclaim"
	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func TestReclaimFailureShowsNextActionWithoutReplaying(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", lang)
			var out, diagnostic bytes.Buffer
			calls := 0
			code := reclaimCommand(context.Background(), []string{"--yes"}, strings.NewReader(""), &out, &diagnostic,
				func(context.Context) (reclamation.WSLTarget, error) { return commandReclaimTarget, nil },
				func(context.Context, reclamation.WSLTarget, string) ([]byte, error) {
					calls++
					return nil, &reclaimclient.InvocationError{Failure: reclamation.InvocationFailure{Phase: "prepare", Stage: "disk_access", NativeError: 5}}
				})
			want := "managed disk access"
			if lang == "ja" {
				want = "管理対象ディスクへのアクセス"
			}
			if code != 1 || calls != 1 || !strings.Contains(diagnostic.String(), want) || !strings.Contains(diagnostic.String(), "haco doctor") || !strings.Contains(diagnostic.String(), "5") {
				t.Fatal(code, calls, diagnostic.String())
			}
		})
	}
}
func TestReclaimAttachedExplainsPreservedDataAndExplicitReview(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	var out bytes.Buffer
	raw := `{"operation":"` + commandReclaimOperation + `","state":"failed","observation":{"Failure":"compact_attached","StopAttempted":true,"StopRequested":true,"ResumeAttempted":true,"Resumed":true}}`
	if code := writeReclamationStatus(&out, io.Discard, []byte(raw)); code != 1 || !strings.Contains(out.String(), "圧縮は始めていません") || !strings.Contains(out.String(), "haco reclaim --review") {
		t.Fatal(code, out.String())
	}
}

func TestLaunchJobObservationRetainsUncertainGuidanceAndSingleDispatch(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", lang)
			t.Setenv("HACO_LOG_FORMAT", "json")
			noJob := reclamation.JobObservation{State: "not_in_job"}
			unknown := reclamation.JobObservation{State: "unavailable", UnavailableAt: "limits", NativeError: 32}
			failure := &reclaimclient.InvocationError{Failure: reclamation.InvocationFailure{Phase: "launch", Stage: "process_start", NativeError: 5, Job: &reclamation.LaunchJobObservation{BeforeStart: noJob, AfterFailure: unknown, OuterJobs: "unknown"}}}
			var out, diagnostic bytes.Buffer
			calls := 0
			code := reclaimCommand(context.Background(), []string{"--yes"}, strings.NewReader(""), &out, &diagnostic,
				func(context.Context) (reclamation.WSLTarget, error) { return commandReclaimTarget, nil },
				func(_ context.Context, _ reclamation.WSLTarget, mode string) ([]byte, error) {
					calls++
					if mode != "start" {
						t.Fatal(mode)
					}
					return nil, failure
				})
			text := diagnostic.String()
			guidance := "A failed launch response does not prove the worker never started."
			if lang == "ja" {
				guidance = "起動応答の失敗だけでは、処理が始まっていないと断定できません。"
			}
			for _, want := range []string{guidance, `"scope":"launcher_immediate_job"`, `"outer_jobs":"unknown"`, `"before_start":{"state":"not_in_job"}`, `"after_failure":{"state":"unavailable","unavailable_at":"limits","native_error":32}`, cliMessage("reclaim.native_error", uint32(5)), cliLanguage().Text("reclaim.text.unconfirmed")} {
				if !strings.Contains(text, want) {
					t.Fatal("missing", want, text)
				}
			}
			if code != 1 || calls != 1 || strings.Contains(text, `"breakaway_ok"`) || strings.Contains(out.String(), "Worker dispatched") {
				t.Fatal(code, calls, out.String(), text)
			}
			// Logger configuration failure never masks the original failure or guidance.
			t.Setenv("HACO_LOG_FORMAT", "unknown")
			diagnostic.Reset()
			writeReclaimInvocationFailure(&diagnostic, failure)
			if !strings.Contains(diagnostic.String(), guidance) || !strings.Contains(diagnostic.String(), cliMessage("reclaim.native_error", uint32(5))) {
				t.Fatal(diagnostic.String())
			}
		})
	}
}
