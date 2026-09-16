package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestConfiguredStorageProviderResolvesOnceAndPinsOwnedPool(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	providerCalls := 0
	if err := runtime.ConfigureStorageProvider(func(context.Context) (map[string]string, error) {
		providerCalls++
		return map[string]string{
			"incus_pool": "haco-lazy-pool",
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if providerCalls != 0 {
		t.Fatalf("storage provider called during configuration: %d", providerCalls)
	}

	for range 2 {
		if pool, err := runtime.defaultRootPool(context.Background()); err != nil || pool != "haco-lazy-pool" {
			t.Fatal("root storage selection changed", pool, err)
		}
	}
	if providerCalls != 1 {
		t.Fatalf("storage provider calls = %d, want 1", providerCalls)
	}

	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "profile show default --project default") {
			t.Fatalf("lazy Environment fell back to Incus default profile: %#v", runner.calls)
		}
	}
}
