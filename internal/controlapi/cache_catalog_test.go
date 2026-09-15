package controlapi

import (
	"context"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
)

func TestCacheCatalogRejectsCallerTargetsAndRequiresReview(t *testing.T) {
	fixture := &cacheWorkflowFixture{}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterCache(s, &cache.Workflow{Catalog: fixture, Collector: fixture}); err != nil {
			t.Fatal(err)
		}
	})
	wire, err := control.NewClient(control.UnixDialer(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []any{
		map[string]string{"operation": "history", "path": "/etc"},
		map[string]string{"operation": "history", "owner": "foreign"},
		map[string]string{"operation": "clear"},
		map[string]string{"operation": "recover", "revision": "short"},
		map[string]string{"operation": "delete", "revision": "short"},
	} {
		if err := wire.Call(context.Background(), MethodCacheCatalog, req, nil); err == nil {
			t.Fatal("invalid catalog request accepted", req)
		}
	}
	if fixture.calls != 0 {
		t.Fatal("invalid request mutated", fixture.calls)
	}
}
