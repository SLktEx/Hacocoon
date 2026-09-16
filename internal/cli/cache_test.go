package cli

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCacheClient struct {
	calls    int
	saved    cache.SettingsSnapshot
	response controlapi.CacheResponse
}

func (f *fakeCacheClient) CacheSettings(context.Context) (cache.SettingsSnapshot, error) {
	f.calls++
	return f.saved, nil
}
func (f *fakeCacheClient) ConfigureCache(_ context.Context, s cache.SettingsSnapshot) (cache.SettingsSnapshot, error) {
	f.calls++
	f.saved = s
	return s, nil
}
func (f *fakeCacheClient) CacheStatus(context.Context, string) (controlapi.CacheResponse, error) {
	f.calls++
	return f.response, nil
}
func (f *fakeCacheClient) CollectCache(context.Context, string, string) (controlapi.CacheResponse, error) {
	f.calls++
	return f.response, nil
}
func TestCacheCommandsShowActionableResultsAndStableJSON(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	f := &fakeCacheClient{response: controlapi.CacheResponse{Areas: []cache.AreaStatus{{Name: "compiler", Path: "/root/.cache/go-build", Origin: 1, Current: 2, State: "published"}}}}
	var out, diagnostic bytes.Buffer
	if code := runCacheWith([]string{"collect", "dev"}, f, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "収集済み") || !strings.Contains(out.String(), "独立コピー") {
		t.Fatal(code, out.String(), diagnostic.String())
	}
	out.Reset()
	f.response.Failure = "recovery_required"
	if code := runCacheWith([]string{"collect", "--json", "dev"}, f, &out, &diagnostic); code != 1 || !strings.Contains(out.String(), `"state":"published"`) || !strings.Contains(diagnostic.String(), "停止したまま") {
		t.Fatal(code, out.String(), diagnostic.String())
	}
}
func TestCacheSettingsValidateBeforeCallingController(t *testing.T) {
	f := &fakeCacheClient{saved: cache.SettingsSnapshot{Revision: strings.Repeat("a", 64)}}
	path := filepath.Join(t.TempDir(), "cache.json")
	var out, diagnostic bytes.Buffer
	if err := os.WriteFile(path, []byte(`{"areas":[{"name":"compiler","path":"/root/.cache/go-build","compatibility":"go-amd64"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runCacheWith([]string{"configure", path}, f, &out, &diagnostic); code != 0 || f.calls != 2 || len(f.saved.Configuration.Areas) != 1 {
		t.Fatal(code, f, diagnostic.String())
	}
	f.calls = 0
	if err := os.WriteFile(path, []byte(`{"areas":[],"areas":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runCacheWith([]string{"configure", path}, f, &out, &diagnostic); code != 2 || f.calls != 0 {
		t.Fatal(code, f.calls)
	}
}
