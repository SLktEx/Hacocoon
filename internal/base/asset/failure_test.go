package baseasset

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type interruptedStore struct {
	Store
	find   func(context.Context, core.BaseRef, string, string) (core.BaseAsset, error)
	begin  func(context.Context, core.BaseAsset) error
	record func(context.Context, core.BaseAsset, string) error
}

func (s interruptedStore) FindBaseAsset(ctx context.Context, base core.BaseRef, provider, scope string) (core.BaseAsset, error) {
	if s.find != nil {
		return s.find(ctx, base, provider, scope)
	}
	return s.Store.FindBaseAsset(ctx, base, provider, scope)
}
func (s interruptedStore) BeginBaseAsset(ctx context.Context, asset core.BaseAsset) error {
	if s.begin != nil {
		return s.begin(ctx, asset)
	}
	return s.Store.BeginBaseAsset(ctx, asset)
}
func (s interruptedStore) RecordBaseAsset(ctx context.Context, asset core.BaseAsset, next string) error {
	if s.record != nil {
		return s.record(ctx, asset, next)
	}
	return s.Store.RecordBaseAsset(ctx, asset, next)
}

func TestEnsureRejectsIncompleteInputBeforeSideEffects(t *testing.T) {
	base := core.BaseRef{Name: "tools", Revision: "revision:one"}
	for _, field := range []string{"service", "store", "backend", "provider", "name", "revision", "scope"} {
		t.Run(field, func(t *testing.T) {
			s := &Service{Store: interruptedStore{}, Backend: &backend{}, Provider: "provider"}
			input, scope := base, "pool"
			switch field {
			case "service":
				s = nil
			case "store":
				s.Store = nil
			case "backend":
				s.Backend = nil
			case "provider":
				s.Provider = ""
			case "name":
				input.Name = ""
			case "revision":
				input.Revision = ""
			case "scope":
				scope = ""
			}
			got, err := s.Ensure(context.Background(), input, scope)
			if !errors.Is(err, core.ErrInvalidArgument) || got != (core.BaseAsset{}) {
				t.Fatalf("accepted incomplete input: %+v, %v", got, err)
			}
		})
	}
}

func TestFailuresBeforeOwnershipNeverCreateProviderMaterial(t *testing.T) {
	ctx := context.Background()
	base := core.BaseRef{Name: "tools", Revision: "revision:one"}
	failure := errors.New("unavailable")
	for _, stage := range []string{"lookup", "plan", "begin", "conflict-unreadable"} {
		t.Run(stage, func(t *testing.T) {
			catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			store := interruptedStore{Store: catalog}
			b := &backend{}
			want := failure
			switch stage {
			case "lookup":
				store.find = func(context.Context, core.BaseRef, string, string) (core.BaseAsset, error) {
					return core.BaseAsset{}, failure
				}
			case "plan":
				b.planErr = failure
			case "begin":
				store.begin = func(context.Context, core.BaseAsset) error { return failure }
			case "conflict-unreadable":
				want = core.ErrAlreadyExists
				store.begin = func(context.Context, core.BaseAsset) error { return core.ErrAlreadyExists }
			}
			got, err := (&Service{Store: store, Backend: b, Provider: "provider"}).Ensure(ctx, base, "pool")
			if !errors.Is(err, want) || got != (core.BaseAsset{}) || b.creates != 0 {
				t.Fatal(got, err, b.creates)
			}
			if _, err := catalog.FindBaseAsset(ctx, base, "provider", "pool"); !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("failure created an ownership record: %v", err)
			}
		})
	}
}

func TestConcurrentBeginReusesOnlyVerifiedWinner(t *testing.T) {
	ctx := context.Background()
	base := core.BaseRef{Name: "tools", Revision: "revision:one"}
	for _, winnerState := range []string{"planned", "ready"} {
		t.Run(winnerState, func(t *testing.T) {
			catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			winner := core.BaseAsset{ID: "base-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Base: base, Provider: "provider", Scope: "pool", NativeRef: "owned/winner", Binding: `{"version":1}`, State: "planned"}
			store := interruptedStore{Store: catalog, begin: func(ctx context.Context, _ core.BaseAsset) error {
				if err := catalog.BeginBaseAsset(ctx, winner); err != nil {
					t.Fatal(err)
				}
				if winnerState == "ready" {
					if err := catalog.RecordBaseAsset(ctx, winner, "created"); err != nil {
						t.Fatal(err)
					}
					winner.State = "created"
					if err := catalog.RecordBaseAsset(ctx, winner, "ready"); err != nil {
						t.Fatal(err)
					}
					winner.State = "ready"
				}
				return core.ErrAlreadyExists
			}}
			verified := 0
			b := &backend{verify: func(a core.BaseAsset) error {
				verified++
				if a != winner {
					t.Fatal("substituted winner")
				}
				return nil
			}}
			got, err := (&Service{Store: store, Backend: b, Provider: "provider"}).Ensure(ctx, base, "pool")
			if got != winner || b.creates != 0 {
				t.Fatalf("lost winner or recreated material: %+v", got)
			}
			if winnerState == "ready" && (err != nil || verified != 1) {
				t.Fatal(err, verified)
			}
			if winnerState == "planned" && (!errors.Is(err, core.ErrRecoveryRequired) || verified != 0) {
				t.Fatal(err, verified)
			}
		})
	}
}

func TestReadyWriteFailureRetainsCreationReceiptForRetry(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	catalog := state.NewEnvironmentJSONStore(path)
	base := core.BaseRef{Name: "tools", Revision: "revision:one"}
	failure := errors.New("disk full")
	store := interruptedStore{Store: catalog, record: func(ctx context.Context, a core.BaseAsset, next string) error {
		if next == "ready" {
			return failure
		}
		return catalog.RecordBaseAsset(ctx, a, next)
	}}
	b := &backend{create: func(core.BaseAsset) error { return nil }, verify: func(core.BaseAsset) error { return nil }}
	s := &Service{Store: store, Backend: b, Provider: "provider"}
	got, err := s.Ensure(ctx, base, "pool")
	if !errors.Is(err, failure) || !errors.Is(err, core.ErrRecoveryRequired) || got.State != "created" {
		t.Fatal(got, err)
	}
	reopened := state.NewEnvironmentJSONStore(path)
	if durable, err := reopened.FindBaseAsset(ctx, base, "provider", "pool"); err != nil || durable != got {
		t.Fatal("lost creation receipt", durable, err)
	}
	s.Store = reopened
	again, err := s.Ensure(ctx, base, "pool")
	got.State = "ready"
	if err != nil || again != got || b.creates != 1 {
		t.Fatal("retry recreated material", again, err, b.creates)
	}
}
