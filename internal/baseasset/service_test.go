package baseasset

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type backend struct {
	store   *state.EnvironmentJSONStore
	create  func(core.BaseAsset) error
	verify  func(core.BaseAsset) error
	creates int
}

func (b *backend) Plan(_ context.Context, _ core.BaseRef, _, owner string) (string, string, error) {
	return "owned/" + owner, `{"version":1}`, nil
}
func (b *backend) Create(_ context.Context, a core.BaseAsset) error { b.creates++; return b.create(a) }
func (b *backend) Verify(_ context.Context, a core.BaseAsset) error { return b.verify(a) }

type failingReceipt struct{ Store }

func (s failingReceipt) RecordBaseAsset(context.Context, core.BaseAsset, string) error {
	return errors.New("write failed")
}

func TestEnsureBaseAssetDurableOrderingFailuresAndReuse(t *testing.T) {
	for _, mode := range []string{"ok", "create-failed", "receipt-failed", "verify-failed", "reuse-failed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "state.json")
			catalog := state.NewEnvironmentJSONStore(path)
			base := core.BaseRef{Name: "dev/base", Revision: "revision:one"}
			verified := 0
			b := &backend{store: catalog}
			b.create = func(a core.BaseAsset) error {
				// Reopen the actual file, not the coordinator's in-memory state.
				got, err := state.NewEnvironmentJSONStore(path).FindBaseAsset(ctx, base, "provider", "pool")
				if err != nil || got != a || got.State != "planned" {
					t.Fatal("create without durable plan", err)
				}
				if mode == "create-failed" {
					return errors.New("ambiguous create")
				}
				return nil
			}
			b.verify = func(a core.BaseAsset) error {
				verified++
				got, err := state.NewEnvironmentJSONStore(path).FindBaseAsset(ctx, base, "provider", "pool")
				if err != nil || got != a || (got.State != "created" && got.State != "ready") {
					t.Fatal("verify without receipt", err)
				}
				if mode == "verify-failed" || (mode == "reuse-failed" && verified == 2) {
					return errors.New("ownership mismatch")
				}
				return nil
			}
			s := Service{Store: catalog, Backend: b, Provider: "provider"}
			if mode == "receipt-failed" {
				s.Store = failingReceipt{catalog}
			}
			got, err := s.Ensure(ctx, base, "pool")
			if mode == "ok" || mode == "reuse-failed" {
				if err != nil || got.State != "ready" {
					t.Fatal(got, err)
				}
				again, err := s.Ensure(ctx, base, "pool")
				if mode == "ok" && (err != nil || again != got) {
					t.Fatal("reuse", again, err)
				}
				if mode == "reuse-failed" && !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, core.ErrRecoveryRequired) || got.ID == "" {
					t.Fatal("lost failure identity", got, err)
				}
				durable, readErr := catalog.FindBaseAsset(ctx, base, "provider", "pool")
				if readErr != nil || durable != got {
					t.Fatal("lost ownership", readErr)
				}
				if _, err := s.Ensure(ctx, base, "pool"); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("blind retry", err)
				}
				if mode == "receipt-failed" && verified != 0 {
					t.Fatal("provider called after failed receipt")
				}
			}
			if b.creates != 1 {
				t.Fatal("duplicate create", b.creates)
			}
		})
	}
}
