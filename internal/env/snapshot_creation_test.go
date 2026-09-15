package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type savedReceiptProvider struct{ receiptTestProvider }

func (p *savedReceiptProvider) CreateEnvironmentFromSnapshot(ctx context.Context, spec core.EnvironmentRuntimeSpec, saved core.Snapshot, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	return p.CreateEnvironmentWithReceipt(ctx, spec, record)
}

func TestSnapshotCreationRejectsUnverifiedOrAmbiguousOwnership(t *testing.T) {
	p := &savedReceiptProvider{receiptTestProvider{baseTestProvider: baseTestProvider{created: core.EnvironmentRuntime{Ref: "created"}}}}
	r, err := NewRouter(testProvider, Register(testProvider, p))
	if err != nil {
		t.Fatal(err)
	}
	limited, _ := NewRouter(testProvider, Register(testProvider, &fakeProvider{}))
	root := core.SnapshotComponent{Role: "rootfs", State: "verified", NativeRef: encodeRouteRef(testProvider, "source/saved")}
	valid := core.Snapshot{State: "ready", Components: []core.SnapshotComponent{root}}
	unverified, missing := root, root
	unverified.State = "creating"
	missing.NativeRef = encodeRouteRef("missing", "source/saved")
	for _, tc := range []struct {
		name       string
		router     *Router
		spec       core.EnvironmentRuntimeSpec
		saved      core.Snapshot
		omitRecord bool
		want       error
	}{
		{"nil-router", nil, core.EnvironmentRuntimeSpec{}, valid, false, core.ErrInvalidArgument},
		{"missing-recorder", r, core.EnvironmentRuntimeSpec{}, valid, true, core.ErrInvalidArgument},
		{"creating", r, core.EnvironmentRuntimeSpec{}, core.Snapshot{State: "creating", Components: valid.Components}, false, core.ErrInvalidArgument},
		{"base-substitution", r, core.EnvironmentRuntimeSpec{Base: "different-base"}, valid, false, core.ErrInvalidArgument},
		{"temporary-substitution", r, core.EnvironmentRuntimeSpec{TemporaryWorkspace: true}, valid, false, core.ErrInvalidArgument},
		{"missing-root", r, core.EnvironmentRuntimeSpec{}, core.Snapshot{State: "ready"}, false, core.ErrIncompatibleState},
		{"duplicate-root", r, core.EnvironmentRuntimeSpec{}, core.Snapshot{State: "ready", Components: []core.SnapshotComponent{root, root}}, false, core.ErrIncompatibleState},
		{"unverified-root", r, core.EnvironmentRuntimeSpec{}, core.Snapshot{State: "ready", Components: []core.SnapshotComponent{unverified}}, false, core.ErrIncompatibleState},
		{"missing-owner", r, core.EnvironmentRuntimeSpec{}, core.Snapshot{State: "ready", Components: []core.SnapshotComponent{missing}}, false, core.ErrUnsupported},
		{"unsupported-owner", limited, core.EnvironmentRuntimeSpec{}, valid, false, core.ErrUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			record := func(core.EnvironmentRuntime) error { calls++; return nil }
			if tc.omitRecord {
				record = nil
			}
			if _, err := tc.router.CreateEnvironmentFromSnapshot(context.Background(), tc.spec, tc.saved, record); !errors.Is(err, tc.want) || calls != 0 {
				t.Fatal("invalid saved ownership created a receipt", err, calls)
			}
		})
	}
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
			result, err := router.CreateEnvironmentFromSnapshot(context.Background(), core.EnvironmentRuntimeSpec{}, saved, func(v core.EnvironmentRuntime) error {
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
