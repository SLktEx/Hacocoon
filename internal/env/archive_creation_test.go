package environment

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"strings"
	"testing"
)

type archiveReceiptProvider struct{ receiptTestProvider }

func (p *archiveReceiptProvider) CreateEnvironmentFromArchive(ctx context.Context, spec core.EnvironmentRuntimeSpec, source io.ReadSeeker, privateRoot string, limit int64, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	return p.CreateEnvironmentWithReceipt(ctx, spec, record)
}
func TestArchiveReceiptRoutesAndRejectsProtocolDrift(t *testing.T) {
	for _, mode := range []string{"ok", "omitted", "duplicate", "drift"} {
		t.Run(mode, func(t *testing.T) {
			p := &archiveReceiptProvider{receiptTestProvider{baseTestProvider: baseTestProvider{created: core.EnvironmentRuntime{Ref: "haco-demo"}}, mode: mode}}
			router, err := NewRouter(ProviderIncus, Register(ProviderIncus, p))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			result, err := router.CreateEnvironmentFromArchive(context.Background(), core.EnvironmentRuntimeSpec{}, strings.NewReader("native archive"), t.TempDir(), 1024, func(v core.EnvironmentRuntime) error {
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

func (p *archiveReceiptProvider) SupportsTemporaryWorkspace() bool { return true }
func TestArchiveRoutesTemporaryWorkspaceWithoutHostPath(t *testing.T) {
	p := &archiveReceiptProvider{receiptTestProvider{baseTestProvider: baseTestProvider{created: core.EnvironmentRuntime{Ref: "haco-builder"}}, mode: "ok"}}
	router, err := NewRouter(ProviderIncus, Register(ProviderIncus, p))
	if err != nil {
		t.Fatal(err)
	}
	work := core.NewTemporaryWorkspace()
	spec := core.EnvironmentRuntimeSpec{TemporaryWorkspace: true, WorkspacePath: work.Path}
	recorded := 0
	_, err = router.CreateEnvironmentFromArchive(context.Background(), spec, strings.NewReader("archive"), t.TempDir(), 1024, func(core.EnvironmentRuntime) error { recorded++; return nil })
	if err != nil || recorded != 1 {
		t.Fatal("temporary archive not routed", err, recorded)
	}
	spec.WorkspacePath = "/host/private"
	_, err = router.CreateEnvironmentFromArchive(context.Background(), spec, strings.NewReader("archive"), t.TempDir(), 1024, func(core.EnvironmentRuntime) error { recorded++; return nil })
	if err == nil || recorded != 1 {
		t.Fatal("Host path accepted as temporary", err, recorded)
	}
}
