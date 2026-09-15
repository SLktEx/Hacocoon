package cache

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type maintenanceFixture struct {
	env         core.Environment
	generation  core.ResourceGeneration
	resources   []core.PersistentResource
	deleted     []core.PersistentResourceRef
	busy        string
	resets      int
	beforeReset func()
}

func (f *maintenanceFixture) GetEnvironment(context.Context, string) (core.Environment, error) {
	return f.env, nil
}
func (f *maintenanceFixture) GetResourceGeneration(context.Context, string) (core.ResourceGeneration, error) {
	return f.generation, nil
}
func (f *maintenanceFixture) ListPersistentResources(context.Context) ([]core.PersistentResource, error) {
	return f.resources, nil
}
func (f *maintenanceFixture) ResetResourceGeneration(_ context.Context, expected core.ResourceGeneration, _ string) (core.ResourceGeneration, error) {
	if f.beforeReset != nil {
		f.beforeReset()
	}
	if f.generation != expected {
		return f.generation, core.ErrSourceGenerationStale
	}
	f.resets++
	f.generation.Number = 0
	f.generation.Current = core.PersistentResourceRef{}
	f.generation.Epoch = strings.Repeat("f", 32)
	return f.generation, nil
}
func (f *maintenanceFixture) DeleteUnselectedGeneration(_ context.Context, ref core.PersistentResourceRef) error {
	f.deleted = append(f.deleted, ref)
	if ref.ID == f.busy {
		return core.ErrStorageBusy
	}
	return nil
}
func maintenanceTestWorkflow() (*Workflow, *maintenanceFixture) {
	origin := core.ResourceGeneration{Name: "compiler", Kind: Kind, Compatibility: strings.Repeat("a", 64), Epoch: strings.Repeat("b", 32)}
	r := core.PersistentResource{ID: "generation:first", Owner: strings.Repeat("c", 32), Kind: Kind, State: "ready", SourceOnly: true, PublicationOrigin: origin}
	current := origin
	current.Number = 1
	current.Current = r.Ref()
	f := &maintenanceFixture{env: core.Environment{Name: "dev", Attachments: []core.EnvironmentAttachment{{Key: "compiler", Target: "/root/.cache/compiler", Origin: origin}}}, generation: current, resources: []core.PersistentResource{r}}
	return &Workflow{Catalog: f, Cleaner: f}, f
}
func TestCacheClearRefusesChangedReviewAndConcurrentPublication(t *testing.T) {
	for _, when := range []string{"review", "reset"} {
		t.Run(when, func(t *testing.T) {
			w, f := maintenanceTestWorkflow()
			h, err := w.History(context.Background(), "dev", "compiler")
			if err != nil {
				t.Fatal(err)
			}
			change := func() { f.generation.Number++ }
			if when == "review" {
				change()
			} else {
				f.beforeReset = change
			}
			r, err := w.Clear(context.Background(), "dev", "compiler", h.Revision)
			if err == nil || r.Reset || len(f.deleted) != 0 || f.resets != 0 {
				t.Fatal(r, err, f)
			}
		})
	}
}
func TestCacheClearOnlyReviewedSourcesAndRetainsUncertainty(t *testing.T) {
	w, f := maintenanceTestWorkflow()
	pending := f.resources[0]
	pending.ID = "generation:pending"
	pending.State = "creating"
	busy := f.resources[0]
	busy.ID = "generation:busy"
	f.busy = busy.ID
	foreign := f.resources[0]
	foreign.ID = "generation:foreign"
	foreign.PublicationOrigin.Name = "other"
	child := f.resources[0]
	child.ID = "env-data:child"
	child.SourceOnly = false
	child.EnvironmentInstance = "env-child"
	oci := f.resources[0]
	oci.ID = "oci:work"
	oci.Kind = "oci-store"
	f.resources = append(f.resources, pending, busy, foreign, child, oci)
	h, err := w.History(context.Background(), "dev", "compiler")
	if err != nil || len(h.Entries) != 3 {
		t.Fatal(h, err)
	}
	r, err := w.Clear(context.Background(), "dev", "compiler", h.Revision)
	if !r.Reset || !errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, core.ErrStorageBusy) || len(f.deleted) != 2 {
		t.Fatal(r, err, f.deleted)
	}
	states := map[string]int{}
	for _, e := range r.Entries {
		states[e.State]++
	}
	if states["deleted"] != 1 || states["cleanup-required"] != 1 || states["recovery-required"] != 1 {
		t.Fatal(states)
	}
	for _, ref := range f.deleted {
		if ref != f.resources[0].Ref() && ref != busy.Ref() {
			t.Fatal("unreviewed resource deleted", ref)
		}
	}
	if f.generation.Epoch == f.env.Attachments[0].Origin.Epoch || f.generation.Number != 0 {
		t.Fatal("old producer not fenced")
	}
}
func TestCacheHistoryDoesNotClaimUnselectedCandidateWasPublished(t *testing.T) {
	w, f := maintenanceTestWorkflow()
	retained := f.resources[0]
	retained.ID = "generation:unselected"
	f.resources = append(f.resources, retained)
	h, err := w.History(context.Background(), "dev", "compiler")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Entries) != 2 || h.Entries[0].State != "current" || h.Entries[1].State != "retained" {
		t.Fatal(h)
	}
	before := h.Revision
	f.resources[1].Owner = strings.Repeat("d", 32)
	h, err = w.History(context.Background(), "dev", "compiler")
	if err != nil || h.Revision == before {
		t.Fatal("ownership not review-bound", h, err)
	}
}
