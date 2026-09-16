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
