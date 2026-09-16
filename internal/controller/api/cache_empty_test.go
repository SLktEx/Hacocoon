package controlapi

import (
	"context"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
)

func TestCacheEmptyTransportRejectsProviderOverridesAndMixedScope(t *testing.T) {
	path := doctorTestSocket(t, func(s *control.Server) {
		fixture := &cacheWorkflowFixture{}
		if err := RegisterCache(s, &cache.Workflow{Catalog: fixture, Collector: fixture}); err != nil {
			t.Fatal(err)
		}
	})
	wire, err := control.NewClient(control.UnixDialer(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{MethodCacheEmptyPreview, MethodCacheEmpty} {
		for _, request := range []any{
			map[string]any{"scope": map[string]any{"environment": "dev", "path": "/etc"}},
			map[string]any{"scope": map[string]any{"environment": "dev", "all": true}},
			map[string]any{"scope": map[string]any{"environment": "dev"}, "owner": "foreign"},
			map[string]any{"scope": map[string]any{"all": true}, "revision": "short"},
		} {
			if err := wire.Call(context.Background(), method, request, nil); err == nil {
				t.Fatal("invalid request accepted", method, request)
			}
		}
	}
}
