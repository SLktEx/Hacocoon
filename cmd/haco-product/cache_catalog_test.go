package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
)

type catalogClientFixture struct {
	maintenanceClientFixture
	calls []string
}

func (f *catalogClientFixture) MaintainCacheCatalog(_ context.Context, operation, revision string) (controlapi.CacheCatalogResponse, error) {
	f.calls = append(f.calls, operation)
	if operation != "history" && revision != strings.Repeat("b", 64) {
		panic("wrong review")
	}
	return controlapi.CacheCatalogResponse{Catalog: cache.CatalogHistory{Revision: strings.Repeat("b", 64), Groups: []cache.CatalogGroup{{History: cache.History{Entries: []cache.HistoryEntry{{State: "current"}}}}}}}, nil
}
func TestCacheAllClearRequiresCommonConfirmationAndReadableScope(t *testing.T) {
	for _, tc := range []struct {
		name       string
		diagnostic io.Writer
		want       int
	}{{"shown", new(bytes.Buffer), 2}, {"display-failed", failedCacheWriter{}, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &catalogClientFixture{}
			var out bytes.Buffer
			code := runCacheMaintenance([]string{"clear", "--all", "--yes"}, f, unexpectedConfirmationRead{t}, &out, tc.diagnostic)
			if len(f.calls) != tc.want || tc.want == 2 && code != 0 {
				t.Fatal(code, f.calls)
			}
		})
	}
}
func TestCacheAllHistoryDoesNotNeedProducerOrOpaqueIdentifier(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	f := &catalogClientFixture{}
	var out, diagnostic bytes.Buffer
	code := runCacheMaintenance([]string{"history", "--all"}, f, unexpectedConfirmationRead{t}, &out, &diagnostic)
	if code != 0 || len(f.calls) != 1 || !strings.Contains(out.String(), "既存Envはありません") || strings.Contains(out.String(), strings.Repeat("b", 64)) {
		t.Fatal(code, out.String(), f.calls)
	}
	if code := runCacheMaintenance([]string{"clear", "--all", "dev", "compiler"}, f, unexpectedConfirmationRead{t}, &out, &diagnostic); code != 2 {
		t.Fatal("mixed scope accepted", code)
	}
}
