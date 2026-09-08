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

func TestEnsureRecoversDurablyCreatedBaseAfterRestart(t *testing.T) {
	for _, mode := range []string{"created", "planned", "changed-material", "publish-failed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "state.json")
			catalog := state.NewEnvironmentJSONStore(path)
			a := core.BaseAsset{ID: "base-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Base: core.BaseRef{Name: "dev/base", Revision: "revision:one"}, Provider: "provider", Scope: "pool", NativeRef: "owned/base", Binding: `{"version":1}`, State: "planned"}
			if err := catalog.BeginBaseAsset(ctx, a); err != nil {
				t.Fatal(err)
			}
			if mode != "planned" {
				if err := catalog.RecordBaseAsset(ctx, a, "created"); err != nil {
					t.Fatal(err)
				}
				a.State = "created"
			}
			reopened := state.NewEnvironmentJSONStore(path)
			verified := 0
			b := &backend{create: func(core.BaseAsset) error { t.Fatal("recovery recreated Base"); return nil }, verify: func(got core.BaseAsset) error {
				verified++
				if got != a {
					t.Fatal("recovery substituted ownership")
				}
				if mode == "changed-material" {
					return errors.New("material changed")
				}
				return nil
			}}
			service := Service{Store: reopened, Backend: b, Provider: "provider"}
			if mode == "publish-failed" {
				service.Store = failingReceipt{reopened}
			}
			got, err := service.Ensure(ctx, a.Base, a.Scope)
			durable, readErr := state.NewEnvironmentJSONStore(path).FindBaseAsset(ctx, a.Base, a.Provider, a.Scope)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "created" {
				a.State = "ready"
				if err != nil || got != a || durable != a {
					t.Fatal("recovery did not finish exact asset", got, err)
				}
			} else {
				if !errors.Is(err, core.ErrRecoveryRequired) || got != a || durable != a {
					t.Fatal("lost incomplete ownership", got, err)
				}
			}
			if b.creates != 0 || (mode == "planned" && verified != 0) || (mode != "planned" && verified != 1) {
				t.Fatal("invalid recovery provider operations", b.creates, verified)
			}
		})
	}
}

func TestConcurrentBaseRecoveryConvergesOnExactReadyAsset(t *testing.T) {
	ctx := context.Background()
	catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	a := core.BaseAsset{ID: "base-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Base: core.BaseRef{Name: "dev/base", Revision: "revision:one"}, Provider: "provider", Scope: "pool", NativeRef: "owned/base", Binding: `{"version":1}`, State: "planned"}
	if err := catalog.BeginBaseAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := catalog.RecordBaseAsset(ctx, a, "created"); err != nil {
		t.Fatal(err)
	}
	a.State = "created"
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	b := &backend{verify: func(core.BaseAsset) error { arrived <- struct{}{}; <-release; return nil }}
	service := Service{Store: catalog, Backend: b, Provider: "provider"}
	for i := 0; i < 2; i++ {
		go func() {
			got, err := service.Ensure(ctx, a.Base, a.Scope)
			if err == nil && got.State != "ready" {
				err = errors.New("not ready")
			}
			results <- err
		}()
	}
	<-arrived
	<-arrived
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
