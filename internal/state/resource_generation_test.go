package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func generationFixture(t *testing.T, s *EnvironmentJSONStore, letter string) core.PersistentResource {
	t.Helper()
	r := core.PersistentResource{ID: "generation:" + strings.Repeat(letter, 32), Owner: strings.Repeat(letter, 32), Kind: "build-cache", SourceOnly: true, NativeRef: "pool/" + letter, State: "creating", CreatedAt: time.Now().UTC()}
	if err := s.BeginPersistentResourceCreate(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := s.CommitPersistentResourceCreate(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	r.State = "ready"
	return r
}

func TestResourceGenerationConcurrentAdoptionAndReset(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	a, b := NewEnvironmentJSONStore(path), NewEnvironmentJSONStore(path)
	initial, err := a.EnsureResourceGeneration(ctx, "compiler", "build-cache", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	again, err := b.EnsureResourceGeneration(ctx, initial.Name, initial.Kind, initial.Compatibility)
	if err != nil || again != initial {
		t.Fatal(again, err)
	}
	if _, err := b.EnsureResourceGeneration(ctx, initial.Name, initial.Kind, strings.Repeat("b", 64)); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
	candidates := []core.PersistentResource{generationFixture(t, a, "a"), generationFixture(t, b, "b")}
	stores := []*EnvironmentJSONStore{a, b}
	outcomes := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range stores {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, outcomes[i] = stores[i].AdvanceResourceGeneration(ctx, initial, candidates[i].Ref())
		}(i)
	}
	close(start)
	wg.Wait()
	successes, stale := 0, 0
	for _, err := range outcomes {
		if err == nil {
			successes++
		} else if errors.Is(err, core.ErrSourceGenerationStale) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || stale != 1 {
		t.Fatal(outcomes)
	}
	current, err := a.GetResourceGeneration(ctx, initial.Name)
	if err != nil || current.Number != 1 {
		t.Fatal(current, err)
	}
	for _, call := range []func() error{
		func() error { _, err := a.BeginPersistentResourceDelete(ctx, current.Current.ID); return err },
		func() error { _, err := b.BeginGenerationResourceDelete(ctx, current.Current); return err },
		func() error { _, err := a.BeginPersistentResourceDeleteReviewed(ctx, current.Current); return err },
		func() error {
			r, err := a.GetPersistentResource(ctx, current.Current.ID)
			if err != nil {
				return err
			}
			r.State = "deleting"
			return a.FinalizePersistentResourceDelete(ctx, r)
		},
	} {
		if err := call(); !errors.Is(err, core.ErrStorageBusy) {
			t.Fatalf("current source deletion: %v", err)
		}
	}
	reset, err := b.ResetResourceGeneration(ctx, current, strings.Repeat("b", 64))
	if err != nil || reset.Number != 0 || reset.Epoch == current.Epoch || reset.Current != (core.PersistentResourceRef{}) {
		t.Fatal(reset, err)
	}
	for _, old := range []core.ResourceGeneration{initial, current} {
		if _, err := a.AdvanceResourceGeneration(ctx, old, candidates[0].Ref()); !errors.Is(err, core.ErrSourceGenerationStale) {
			t.Fatal(err)
		}
	}
	resources, err := a.ListPersistentResources(ctx)
	if err != nil || len(resources) != 2 {
		t.Fatal(resources, err)
	}
	for _, r := range resources {
		if r.State != "ready" {
			t.Fatal(r)
		}
	}
	if _, err := a.BeginGenerationResourceDelete(ctx, current.Current); err != nil {
		t.Fatal(err)
	}
}

func TestResourceGenerationCatalogVersionsPreserveOldData(t *testing.T) {
	for _, version := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, -1} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			before := []byte(fmt.Sprintf(`{"version":%d,"environments":{}}`, version))
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			s := NewEnvironmentJSONStore(path)
			_, err := s.ListResourceGenerations(context.Background())
			supported := version == 0 || (version >= 2 && version <= 8) || (version >= 10 && version <= 15)
			if supported && err != nil || !supported && !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("read changed old catalog: %v", err)
			}
		})
	}
	// A real version-14 managed resource survives the first generation write.
	path := filepath.Join(t.TempDir(), "state.json")
	s := NewEnvironmentJSONStore(path)
	r := generationFixture(t, s, "c")
	data, err := s.readEnvironments()
	if err != nil {
		t.Fatal(err)
	}
	data.Version = 14
	writeGenerationFixture(t, path, data)
	if _, err := s.EnsureResourceGeneration(context.Background(), "cache", "build-cache", strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	retained, err := s.GetPersistentResource(context.Background(), r.ID)
	if err != nil || !reflect.DeepEqual(retained, r) {
		t.Fatal(retained, err)
	}
}

func writeGenerationFixture(t *testing.T, path string, data environmentFileState) {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestResourceGenerationCorruptionFailsClosed(t *testing.T) {
	cases := map[string]func(*environmentFileState){
		"old-version":    func(d *environmentFileState) { d.Version = 14 },
		"future-version": func(d *environmentFileState) { d.Version = 16 },
		"wrong-key": func(d *environmentFileState) {
			d.ResourceGenerations["other"] = d.ResourceGenerations["cache"]
			delete(d.ResourceGenerations, "cache")
		},
		"missing-resource": func(d *environmentFileState) { d.PersistentResources = map[string]core.PersistentResource{} },
		"wrong-owner": func(d *environmentFileState) {
			g := d.ResourceGenerations["cache"]
			g.Current.Owner = strings.Repeat("f", 32)
			d.ResourceGenerations["cache"] = g
		},
		"creating": func(d *environmentFileState) {
			for id, r := range d.PersistentResources {
				r.State = "creating"
				d.PersistentResources[id] = r
			}
		},
		"not-source": func(d *environmentFileState) {
			for id, r := range d.PersistentResources {
				r.SourceOnly = false
				d.PersistentResources[id] = r
			}
		},
		"wrong-kind": func(d *environmentFileState) {
			for id, r := range d.PersistentResources {
				r.Kind = "oci-containerd"
				d.PersistentResources[id] = r
			}
		},
		"shared-current": func(d *environmentFileState) {
			g := d.ResourceGenerations["cache"]
			g.Name = "other"
			d.ResourceGenerations[g.Name] = g
		},
		"bad-epoch": func(d *environmentFileState) {
			g := d.ResourceGenerations["cache"]
			g.Epoch = ""
			d.ResourceGenerations["cache"] = g
		},
		"zero-with-current": func(d *environmentFileState) {
			g := d.ResourceGenerations["cache"]
			g.Number = 0
			d.ResourceGenerations["cache"] = g
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "state.json")
			s := NewEnvironmentJSONStore(path)
			g, err := s.EnsureResourceGeneration(ctx, "cache", "build-cache", strings.Repeat("a", 64))
			if err != nil {
				t.Fatal(err)
			}
			r := generationFixture(t, s, "a")
			if _, err = s.AdvanceResourceGeneration(ctx, g, r.Ref()); err != nil {
				t.Fatal(err)
			}
			d, err := s.readEnvironments()
			if err != nil {
				t.Fatal(err)
			}
			corrupt(&d)
			writeGenerationFixture(t, path, d)
			if _, err = s.ListResourceGenerations(ctx); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
			if _, err = s.EnvironmentInstance(ctx, core.Environment{Name: "absent"}); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
		})
	}
}
