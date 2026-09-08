package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func baseAssetFixture() core.BaseAsset {
	return core.BaseAsset{ID: "base-" + strings.Repeat("a", 32), Owner: strings.Repeat("a", 32), Base: core.BaseRef{Name: "dev/base", Revision: "revision:one"}, Provider: "provider", Scope: "pool", NativeRef: "owned/base-one", Binding: `{"version":1}`, State: "planned"}
}

func TestBaseAssetReceiptRestartAndCAS(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	s := NewEnvironmentJSONStore(path)
	a := baseAssetFixture()
	mustSnapshot(t, s.BeginBaseAsset(ctx, a))
	s = NewEnvironmentJSONStore(path)
	got, err := s.FindBaseAsset(ctx, a.Base, a.Provider, a.Scope)
	mustSnapshot(t, err)
	if got != a {
		t.Fatal("lost planned ownership")
	}
	drift := a
	drift.Binding = `{"version":2}`
	if err := s.RecordBaseAsset(ctx, drift, "created"); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	if err := s.RecordBaseAsset(ctx, a, "ready"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("ready without receipt", err)
	}
	mustSnapshot(t, s.RecordBaseAsset(ctx, a, "created"))
	a.State = "created"
	mustSnapshot(t, s.RecordBaseAsset(ctx, a, "ready"))
	a.State = "ready"
	s = NewEnvironmentJSONStore(path)
	got, err = s.FindBaseAsset(ctx, a.Base, a.Provider, a.Scope)
	mustSnapshot(t, err)
	if got != a {
		t.Fatal("lost ready ownership")
	}
	if err := s.RecordBaseAsset(ctx, a, "planned"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("downgrade", err)
	}
	for _, key := range []core.BaseAsset{{Base: core.BaseRef{Name: a.Base.Name, Revision: "revision:two"}, Provider: a.Provider, Scope: a.Scope}, {Base: a.Base, Provider: "other", Scope: a.Scope}, {Base: a.Base, Provider: a.Provider, Scope: "other"}} {
		if _, err := s.FindBaseAsset(ctx, key.Base, key.Provider, key.Scope); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("substituted asset", err)
		}
	}
}

func TestBaseAssetConcurrentReservationAndNativeCollision(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	a := baseAssetFixture()
	b := a
	b.ID = "base-" + strings.Repeat("b", 32)
	b.Owner = strings.Repeat("b", 32)
	b.NativeRef = "owned/base-two"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, v := range []core.BaseAsset{a, b} {
		wg.Add(1)
		go func(v core.BaseAsset) {
			defer wg.Done()
			results <- NewEnvironmentJSONStore(path).BeginBaseAsset(ctx, v)
		}(v)
	}
	wg.Wait()
	close(results)
	success, collision := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, core.ErrAlreadyExists) {
			collision++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || collision != 1 {
		t.Fatal(success, collision)
	}
	s := NewEnvironmentJSONStore(path)
	current, err := s.FindBaseAsset(ctx, a.Base, a.Provider, a.Scope)
	mustSnapshot(t, err)
	other := current
	other.ID = "base-" + strings.Repeat("c", 32)
	other.Owner = strings.Repeat("c", 32)
	other.Base.Revision = "revision:other"
	if err := s.BeginBaseAsset(ctx, other); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatal("native alias collision", err)
	}
}

func TestBaseAssetSchemaOwnershipCannotBeDowngraded(t *testing.T) {
	for _, version := range []int{0, 2, 3, 4, 5, 6, 7} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			a := baseAssetFixture()
			d := newEnvironmentFileState()
			d.Version = version
			d.BaseAssets[a.ID] = a
			raw, err := json.Marshal(d)
			mustSnapshot(t, err)
			path := filepath.Join(t.TempDir(), "state.json")
			mustSnapshot(t, os.WriteFile(path, raw, 0600))
			_, err = NewEnvironmentJSONStore(path).FindBaseAsset(context.Background(), a.Base, a.Provider, a.Scope)
			if version == 7 {
				mustSnapshot(t, err)
			} else if !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
		})
	}
	// A schema-6 snapshot remains recoverable on upgrade, with its exact binding.
	s, snap := snapshotCatalogFixture(t)
	snap.Components[0].Binding = `{"owned":true}`
	mustSnapshot(t, s.BeginSnapshot(context.Background(), snap))
	raw, err := os.ReadFile(s.path)
	mustSnapshot(t, err)
	var d environmentFileState
	mustSnapshot(t, json.Unmarshal(raw, &d))
	d.Version = 6
	raw, err = json.Marshal(d)
	mustSnapshot(t, err)
	mustSnapshot(t, os.WriteFile(s.path, raw, 0600))
	got, err := NewEnvironmentJSONStore(s.path).GetSnapshot(context.Background(), snap.ID)
	mustSnapshot(t, err)
	if got.Components[0] != snap.Components[0] {
		t.Fatal("schema-6 binding lost")
	}
}

func TestBaseAssetRejectsMalformedOrDuplicatedCatalog(t *testing.T) {
	for _, mode := range []string{"control", "oversized-binding", "empty-binding", "state", "duplicate-scope", "duplicate-native"} {
		t.Run(mode, func(t *testing.T) {
			a := baseAssetFixture()
			d := newEnvironmentFileState()
			switch mode {
			case "control":
				a.Scope = "pool\nforeign"
			case "oversized-binding":
				a.Binding = strings.Repeat("x", 16385)
			case "empty-binding":
				a.Binding = ""
			case "state":
				a.State = "absent"
			default:
				b := a
				b.ID = "base-" + strings.Repeat("b", 32)
				b.Owner = strings.Repeat("b", 32)
				if mode == "duplicate-native" {
					b.Base.Revision = "revision:two"
				} else {
					b.NativeRef = "owned/other"
				}
				d.BaseAssets[b.ID] = b
			}
			d.BaseAssets[a.ID] = a
			if err := normalizeEnvironmentState(&d); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
		})
	}
}
