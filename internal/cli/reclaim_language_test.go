package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func TestJapaneseReclamationKeepsStatusValuesAndMeasurements(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	for _, tc := range []struct {
		raw, want string
		code      int
	}{
		{`{"operation":"","state":"none"}`, "容量回収の保存結果はありません", 0},
		{`{"operation":"` + commandReclaimOperation + `","state":"pending"}`, "実行中・結果未確定", 0},
		{`{"operation":"` + commandReclaimOperation + `","state":"failed"}`, "容量回収: 失敗", 1},
		{`{"operation":"` + commandReclaimOperation + `","state":"complete","observation":{"StopAttempted":true,"StopRequested":true,"ResumeAttempted":true,"Resumed":true,"Compaction":{"Attempted":true,"Completed":true,"Virtual":{"Capacity":1048576},"Before":{"LogicalBytes":4096,"AllocatedBytes":2048},"After":{"LogicalBytes":4096,"AllocatedBytes":1024}}}}`, "回収した容量: 1024 bytes", 0},
	} {
		var out, diagnostic bytes.Buffer
		result, err := parseReclamationStatus([]byte(tc.raw))
		if err != nil {
			t.Fatal(err)
		}
		if result.State == "完了" || result.State == "失敗" {
			t.Fatal("translated a protocol value")
		}
		if code := writeReclamationStatus(&out, &diagnostic, []byte(tc.raw)); code != tc.code || !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), "%!") {
			t.Fatal(code, out.String(), diagnostic.String())
		}
		if result.State == "complete" && !strings.Contains(out.String(), "圧縮完了=はい") {
			t.Fatal(out.String())
		}
	}
	var out bytes.Buffer
	if code := writeReclamationStatus(io.Discard, &out, []byte(`{"state":"complete"}`)); code != 1 || !strings.Contains(out.String(), "保存された容量回収の結果が不正") {
		t.Fatal(code, out.String())
	}
}

func TestJapaneseReclamationReviewKeepsExactConsentAndDoesNotReplay(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	for _, answer := range []string{"yes\n", "n\n"} {
		var out, diagnostic bytes.Buffer
		reads, reviews := 0, 0
		code := reclaimReviewCommand(context.Background(), nil, strings.NewReader(answer), &out, &diagnostic,
			func(context.Context) (reclamation.WSLTarget, error) { return commandReclaimTarget, nil },
			func(_ context.Context, target reclamation.WSLTarget, mode string) ([]byte, error) {
				reads++
				if target != commandReclaimTarget || mode != "status" {
					t.Fatal("review changed target or started work")
				}
				return []byte(`{"operation":"` + commandReclaimOperation + `","state":"failed"}`), nil
			},
			func(_ context.Context, target reclamation.WSLTarget, operation, state string) error {
				reviews++
				if target != commandReclaimTarget || operation != commandReclaimOperation || state != "failed" {
					t.Fatal("consent changed by translation")
				}
				return nil
			})
		wantReviews := 0
		if answer == "yes\n" {
			wantReviews = 1
		}
		if code != 0 || reads != 1 || reviews != wantReviews || !strings.Contains(out.String(), "続けますか?") {
			t.Fatal(code, reads, reviews, out.String(), diagnostic.String())
		}
		if answer == "yes\n" && !strings.Contains(out.String(), "新しい操作は開始していません") {
			t.Fatal(out.String())
		}
	}
}
