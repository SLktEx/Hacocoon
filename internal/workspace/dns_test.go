package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

func TestCreationPersistsResolverSelection(t *testing.T) {
	for _, mode := range []core.DNSMode{"", core.DNSHost, core.DNSBackend, core.DNSDisabled, "invalid"} {
		t.Run(string(mode), func(t *testing.T) {
			runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
			store := newFakeEnvironmentStore()
			result, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: t.TempDir(), DNSMode: mode})
			if !mode.Valid() {
				if err == nil || runtime.createSpec.Name != "" || len(store.leases) != 0 {
					t.Fatal("invalid mode mutated lifecycle")
				}
				return
			}
			if err != nil || runtime.createSpec.DNSMode != mode || result.DNSMode != mode || store.environments["demo"].DNSMode != mode {
				t.Fatalf("mode lost: %+v %v", result, err)
			}
		})
	}
}
