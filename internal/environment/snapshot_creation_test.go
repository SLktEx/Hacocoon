package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type savedReceiptProvider struct{ receiptTestProvider }

func (p *savedReceiptProvider) CreateEnvironmentFromSnapshot(ctx context.Context, spec core.EnvironmentRuntimeSpec, saved core.Snapshot, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	return p.CreateEnvironmentWithReceipt(ctx, spec, record)
}
func TestSavedRootReceiptRoutesAndRejectsProtocolDrift(t *testing.T) {
	for _, mode := range []string{"ok", "omitted", "duplicate", "drift"} {
		t.Run(mode, func(t *testing.T) {
			p := &savedReceiptProvider{receiptTestProvider{baseTestProvider: baseTestProvider{created: core.EnvironmentRuntime{Ref: "haco-demo"}}, mode: mode}}
			router, err := NewRouter(ProviderIncus, Register(ProviderIncus, p))
			if err != nil {
				t.Fatal(err)
			}
			saved := core.Snapshot{State: "ready", Components: []core.SnapshotComponent{{Role: "rootfs", State: "verified", NativeRef: encodeRouteRef(ProviderIncus, "instance/saved")}}}
			calls := 0
			result, err := NewBaseRouter(router).CreateEnvironmentFromSnapshot(context.Background(), core.EnvironmentRuntimeSpec{}, saved, func(v core.EnvironmentRuntime) error {
				calls++
				if v.Ref != encodeRouteRef(ProviderIncus, "haco-demo") {
					t.Fatal(v)
				}
				return nil
			})
			if mode == "ok" {
				if err != nil || calls != 1 || result.Ref == "" {
					t.Fatal(result, err, calls)
				}
			} else if err == nil {
				t.Fatal("bad protocol accepted")
			}
		})
	}
}
