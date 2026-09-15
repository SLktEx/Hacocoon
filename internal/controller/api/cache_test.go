package controlapi

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
	"strings"
	"testing"
)

type cacheWorkflowFixture struct {
	calls      int
	generation core.ResourceGeneration
}

func (f *cacheWorkflowFixture) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	return core.Environment{Name: name, Attachments: []core.EnvironmentAttachment{{Key: "compiler", Target: "/root/.cache/compiler", Origin: f.generation}}}, nil
}
func (f *cacheWorkflowFixture) GetResourceGeneration(context.Context, string) (core.ResourceGeneration, error) {
	return f.generation, nil
}
func (f *cacheWorkflowFixture) CollectEnvironmentResource(context.Context, string, string) (core.ResourceGenerationPublication, error) {
	f.calls++
	return core.ResourceGenerationPublication{Generation: f.generation, State: "recovery-required"}, errors.Join(core.ErrRecoveryRequired, errors.New("private-provider-output"))
}
func TestCacheTransportRejectsNativeOverridesAndPreservesUncertainty(t *testing.T) {
	fixture := &cacheWorkflowFixture{generation: core.ResourceGeneration{Name: "compiler", Kind: cache.Kind, Compatibility: strings.Repeat("a", 64), Epoch: strings.Repeat("b", 32)}}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterCache(s, &cache.Workflow{Catalog: fixture, Collector: fixture}); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.CollectCache(context.Background(), "dev", "compiler")
	if err != nil || response.Failure != "recovery_required" || len(response.Areas) != 1 || response.Areas[0].State != "recovery-required" {
		t.Fatal(response, err)
	}
	wire, err := control.NewClient(control.UnixDialer(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []any{map[string]string{"environment": "dev", "path": "/etc"}, map[string]string{"environment": "dev", "native_ref": "pool/foreign"}, map[string]string{"environment": "dev", "owner": "foreign"}} {
		if err := wire.Call(context.Background(), MethodCacheCollect, request, nil); err == nil {
			t.Fatal("untrusted override accepted")
		}
	}
	if fixture.calls != 1 {
		t.Fatal("unexpected provider call", fixture.calls)
	}
}
