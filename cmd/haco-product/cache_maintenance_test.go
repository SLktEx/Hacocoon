package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
)

type maintenanceClientFixture struct {
	clearCalls int
	revision   string
}

func (f *maintenanceClientFixture) CacheHistory(context.Context, string, string) (controlapi.CacheMaintenanceResponse, error) {
	return controlapi.CacheMaintenanceResponse{History: cache.History{Name: "compiler", Path: "/root/.cache/compiler", Revision: strings.Repeat("a", 64), Entries: []cache.HistoryEntry{{State: "retained"}}}}, nil
}
func (f *maintenanceClientFixture) ClearCache(_ context.Context, _, _, revision string) (controlapi.CacheMaintenanceResponse, error) {
	f.clearCalls++
	f.revision = revision
	return controlapi.CacheMaintenanceResponse{Result: cache.ClearResult{Reset: true}}, nil
}

type failedCacheWriter struct{}

func (failedCacheWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestCacheClearRequiresDisplayedReviewAndCommonConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		diagnostic io.Writer
		want       int
	}{
		{"no-interactive-input", []string{"clear", "dev", "compiler"}, new(bytes.Buffer), 0},
		{"yes", []string{"clear", "--yes", "dev", "compiler"}, new(bytes.Buffer), 1},
		{"failed-review-display", []string{"clear", "--yes", "dev", "compiler"}, failedCacheWriter{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &maintenanceClientFixture{}
			var out bytes.Buffer
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			if _, err := writer.WriteString("yes\n"); err != nil {
				t.Fatal(err)
			}
			_ = writer.Close()
			code := runCacheMaintenance(tc.args, f, reader, &out, tc.diagnostic)
			if f.clearCalls != tc.want || tc.want == 1 && (code != 0 || f.revision != strings.Repeat("a", 64)) {
				t.Fatal(code, f)
			}
		})
	}
}
func TestCacheHistoryHumanOutputDoesNotRequireInternalIdentity(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	var out, diagnostic bytes.Buffer
	f := &maintenanceClientFixture{}
	code := runCacheMaintenance([]string{"history", "dev", "compiler"}, f, strings.NewReader(""), &out, &diagnostic)
	if code != 0 || f.clearCalls != 0 || !strings.Contains(out.String(), "未選択") || strings.Contains(out.String(), strings.Repeat("a", 64)) {
		t.Fatal(code, out.String())
	}
}
