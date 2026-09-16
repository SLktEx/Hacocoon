package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
)

type emptyCacheFixture struct {
	calls   int
	scope   cache.EmptyScope
	failure string
}

func (f *emptyCacheFixture) PreviewEmptyCache(_ context.Context, s cache.EmptyScope) (controlapi.CacheEmptyResponse, error) {
	f.scope = s
	return controlapi.CacheEmptyResponse{Preview: cache.EmptyPreview{Revision: strings.Repeat("a", 64), Areas: []cache.EmptyArea{{Environment: "work", Name: "compiler", Path: "/root/.cache/compiler", State: "ready", SavedCopies: 2}}}}, nil
}
func (f *emptyCacheFixture) EmptyCache(_ context.Context, s cache.EmptyScope, rev string) (controlapi.CacheEmptyResponse, error) {
	if s != f.scope || rev != strings.Repeat("a", 64) {
		panic("changed review")
	}
	f.calls++
	return controlapi.CacheEmptyResponse{Failure: f.failure, Preview: cache.EmptyPreview{Areas: []cache.EmptyArea{{Environment: "work", Name: "compiler", State: "empty"}}}}, nil
}
func TestCacheEmptyRequiresReviewAndCommonConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		diag        io.Writer
		calls, code int
	}{
		{"preview", []string{"--preview", "work"}, new(bytes.Buffer), 0, 0},
		{"nonterminal", []string{"work"}, new(bytes.Buffer), 0, 2},
		{"yes", []string{"--yes", "work", "compiler"}, new(bytes.Buffer), 1, 0},
		{"all-json", []string{"--yes", "--json", "--all"}, new(bytes.Buffer), 1, 0},
		{"failed-display", []string{"--yes", "work"}, failedCacheWriter{}, 0, 1},
		{"ambiguous-scope", []string{"--all", "work"}, new(bytes.Buffer), 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &emptyCacheFixture{}
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
			code := runCacheEmpty(tc.args, f, reader, &out, tc.diag)
			if code != tc.code || f.calls != tc.calls {
				t.Fatal(code, f.calls, out.String(), tc.diag)
			}
		})
	}
}
func TestCacheEmptyExplainsRetainedDataInBothLanguages(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", lang)
			var out, diag bytes.Buffer
			f := &emptyCacheFixture{failure: "recovery_required"}
			code := runCacheEmpty([]string{"--yes", "--json", "work"}, f, unexpectedConfirmationRead{t}, &out, &diag)
			if code != 1 || f.calls != 1 || !strings.Contains(out.String(), `"failure":"recovery_required"`) || !strings.Contains(diag.String(), "Workspace") || !strings.Contains(diag.String(), "OCI") || strings.Contains(diag.String(), strings.Repeat("a", 64)) {
				t.Fatal(code, out.String(), diag.String())
			}
		})
	}
}
