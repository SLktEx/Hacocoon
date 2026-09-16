//go:build linux

package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
)

// Empty catalogs and missing Environments must not turn a maintenance request
// into a provider-wide cleanup. Any provider call is observable and fails.
type unexpectedCacheMutation struct{ calls atomic.Int32 }

func (f *unexpectedCacheMutation) CollectEnvironmentResource(context.Context, string, string) (core.ResourceGenerationPublication, error) {
	f.calls.Add(1)
	return core.ResourceGenerationPublication{}, core.ErrIncompatibleState
}
func (f *unexpectedCacheMutation) DeleteUnselectedGeneration(context.Context, core.PersistentResourceRef) error {
	f.calls.Add(1)
	return core.ErrIncompatibleState
}
func (f *unexpectedCacheMutation) RecoverEnvironmentGeneration(context.Context, core.PersistentResourceRef) (core.ResourceGenerationPublication, error) {
	f.calls.Add(1)
	return core.ResourceGenerationPublication{}, core.ErrIncompatibleState
}
func (f *unexpectedCacheMutation) EmptyEnvironmentResource(context.Context, string, core.EnvironmentAttachment) error {
	f.calls.Add(1)
	return core.ErrIncompatibleState
}

func TestCacheSettingsWirePreservesConfigurationAndCAS(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	settings := cache.Settings{Path: filepath.Join(root, "settings", "cache.json")}
	catalog := state.NewEnvironmentJSONStore(filepath.Join(root, "state.json"))
	provider := &unexpectedCacheMutation{}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterCache(s, &cache.Workflow{Settings: settings, Catalog: catalog, Collector: provider}); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := client.CacheSettings(ctx)
	if err != nil || len(initial.Revision) != 64 || len(initial.Configuration.Areas) != 0 {
		t.Fatal(initial, err)
	}
	edit := initial
	edit.Configuration = cache.Configuration{Areas: []cache.Area{{Name: "compiler", Path: "/root/.cache/go-build", Compatibility: "go-linux-amd64"}}}
	saved, err := client.ConfigureCache(ctx, edit)
	if err != nil || saved.Revision == initial.Revision || !reflect.DeepEqual(saved.Configuration, edit.Configuration) {
		t.Fatal("configuration receipt lost requested settings", saved, err)
	}
	durable, err := (cache.Settings{Path: settings.Path}).Read(ctx)
	if err != nil || !reflect.DeepEqual(durable, saved) {
		t.Fatal("wire receipt differs from disk", durable, err)
	}
	if _, err := client.ConfigureCache(ctx, edit); err == nil {
		t.Fatal("stale editor replaced newer configuration")
	}
	for _, payload := range []any{
		map[string]any{"revision": saved.Revision, "configuration": map[string]any{"areas": []any{}, "native_path": "/etc"}},
		map[string]any{"revision": saved.Revision, "configuration": map[string]any{"areas": []any{}}, "owner": "foreign"},
		"invalid request",
	} {
		var status *control.StatusError
		if err := client.wire.Call(ctx, MethodCacheConfigure, payload, nil); !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatal("invalid configuration accepted", err)
		}
	}
	if err := client.wire.Call(ctx, MethodCacheSettings, map[string]any{"path": "/etc"}, nil); err == nil {
		t.Fatal("settings path override accepted")
	}
	current, err := client.CacheSettings(ctx)
	if err != nil || !reflect.DeepEqual(current, saved) || provider.calls.Load() != 0 {
		t.Fatal("refused edits altered configuration or provider", current, err)
	}
}

func TestCacheMaintenanceWireBindsReviewAndPreservesMissingTargets(t *testing.T) {
	ctx := context.Background()
	catalogPath := filepath.Join(t.TempDir(), "state.json")
	catalog := state.NewEnvironmentJSONStore(catalogPath)
	source, err := catalog.EnsureResourceGeneration(ctx, "compiler", cache.Kind, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	provider := &unexpectedCacheMutation{}
	workflow := &cache.Workflow{Catalog: catalog, Collector: provider, Cleaner: provider, Recoverer: provider, Emptier: provider}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterCache(s, workflow); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	history, err := client.MaintainCacheCatalog(ctx, "history", "")
	if err != nil || history.Failure != "" || len(history.Catalog.Groups) != 1 || len(history.Catalog.Revision) != 64 {
		t.Fatal(history, err)
	}
	stale := strings.Repeat("b", 64)
	denied, err := client.MaintainCacheCatalog(ctx, "clear", stale)
	if err != nil || denied.Failure != "stale" || denied.Catalog.Groups[0].Result.Reset {
		t.Fatal("stale review mutated source", denied, err)
	}
	unmodified, err := catalog.GetResourceGeneration(ctx, "compiler")
	if err != nil || unmodified != source {
		t.Fatal("stale review changed catalog", unmodified, err)
	}
	recovered, err := client.MaintainCacheCatalog(ctx, "recover", history.Catalog.Revision)
	if err != nil || recovered.Failure != "" || recovered.Catalog.Groups[0].State != "complete" || len(recovered.Catalog.Groups[0].Recovery.Entries) != 0 {
		t.Fatal(recovered, err)
	}
	cleared, err := client.MaintainCacheCatalog(ctx, "clear", history.Catalog.Revision)
	if err != nil || cleared.Failure != "" || !cleared.Catalog.Groups[0].Result.Reset || cleared.Catalog.Groups[0].State != "complete" {
		t.Fatal(cleared, err)
	}
	updated, err := state.NewEnvironmentJSONStore(catalogPath).GetResourceGeneration(ctx, "compiler")
	if err != nil || updated.Epoch == source.Epoch || updated.Number != 0 || updated.Current != (core.PersistentResourceRef{}) {
		t.Fatal("clear did not persist source reset", updated, err)
	}
	for _, operation := range []string{"clear", "recover"} {
		response, err := client.MaintainCacheCatalog(ctx, operation, history.Catalog.Revision)
		if err != nil || response.Failure != "stale" {
			t.Fatal("old review survived reset", operation, response, err)
		}
	}
	for _, method := range []string{MethodCacheStatus, MethodCacheCollect, MethodCacheHistory, MethodCacheClear, MethodCacheRecover} {
		var failure string
		switch method {
		case MethodCacheStatus:
			result, e := client.CacheStatus(ctx, "missing")
			err = e
			failure = result.Failure
		case MethodCacheCollect:
			result, e := client.CollectCache(ctx, "missing", "compiler")
			err = e
			failure = result.Failure
		case MethodCacheHistory:
			result, e := client.CacheHistory(ctx, "missing", "compiler")
			err = e
			failure = result.Failure
		case MethodCacheClear:
			result, e := client.ClearCache(ctx, "missing", "compiler", stale)
			err = e
			failure = result.Failure
		case MethodCacheRecover:
			result, e := client.RecoverCache(ctx, "missing", "compiler")
			err = e
			failure = result.Failure
		}
		if err != nil || failure != "not_found" {
			t.Fatal("missing target was treated as completed", method, failure, err)
		}
	}
	preview, err := client.PreviewEmptyCache(ctx, cache.EmptyScope{All: true})
	if err != nil || preview.Failure != "" || len(preview.Preview.Revision) != 64 || len(preview.Preview.Areas) != 0 {
		t.Fatal(preview, err)
	}
	if response, err := client.EmptyCache(ctx, cache.EmptyScope{All: true}, stale); err != nil || response.Failure != "stale" {
		t.Fatal("empty operation ignored review", response, err)
	}
	if response, err := client.EmptyCache(ctx, cache.EmptyScope{All: true}, preview.Preview.Revision); err != nil || response.Failure != "" || len(response.Preview.Areas) != 0 {
		t.Fatal("empty scope did not remain empty", response, err)
	}
	if provider.calls.Load() != 0 {
		t.Fatal("empty catalog or missing target reached provider", provider.calls.Load())
	}
	for _, method := range []string{MethodCacheHistory, MethodCacheClear, MethodCacheRecover} {
		var status *control.StatusError
		if err := client.wire.Call(ctx, method, json.RawMessage(`{"environment":"missing","area":"compiler","revision":"short","native_ref":"/etc"}`), nil); !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatal("invalid maintenance override accepted", method, err)
		}
	}
}
