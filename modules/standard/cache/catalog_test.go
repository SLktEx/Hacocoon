package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (f *maintenanceFixture) ListResourceGenerations(context.Context) ([]core.ResourceGeneration, error) {
	return []core.ResourceGeneration{f.generation}, nil
}
func (f *maintenanceFixture) ListEnvironments(context.Context) ([]core.Environment, error) {
	if f.env.Name == "" {
		return []core.Environment{}, nil
	}
	return []core.Environment{f.env}, nil
}
func TestCacheCatalogKeepsOrphansReviewBoundAndExcludesIndependentData(t *testing.T) {
	w, f := maintenanceTestWorkflow()
	// Every producer may disappear while a complete generation remains current.
	f.env = core.Environment{}
	child := f.resources[0]
	child.ID = "env-data:independent"
	child.SourceOnly = false
	child.EnvironmentInstance = "child"
	oci := f.resources[0]
	oci.ID = "oci:retained"
	oci.Kind = "oci-store"
	f.resources = append(f.resources, child, oci)
	h, err := w.CatalogHistory(context.Background())
	if err != nil || len(h.Groups) != 1 || h.Groups[0].History.Name != "" || len(h.Groups[0].History.Entries) != 1 {
		t.Fatal(h, err)
	}
	f.generation.Number++
	if _, err := w.MaintainCatalog(context.Background(), h.Revision, true); !errors.Is(err, core.ErrCapabilityStale) || f.resets != 0 || len(f.deleted) != 0 {
		t.Fatal("stale orphan review mutated", err)
	}
	h, err = w.CatalogHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.MaintainCatalog(context.Background(), h.Revision, true)
	if err != nil || !r.Groups[0].Result.Reset || r.Groups[0].State != "complete" || len(f.deleted) != 1 || f.deleted[0] != f.resources[0].Ref() {
		t.Fatal(r, err, f.deleted)
	}
}

func TestCacheCatalogPreservesFailedAndUncertainOwnedAttempts(t *testing.T) {
	w, f := maintenanceTestWorkflow()
	f.env = core.Environment{}
	f.busy = f.resources[0].ID
	unknown := f.resources[0]
	unknown.ID = "generation:unknown"
	unknown.State = "creating"
	f.resources = append(f.resources, unknown)
	h, err := w.CatalogHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.MaintainCatalog(context.Background(), h.Revision, true)
	if !errors.Is(err, core.ErrStorageBusy) || !errors.Is(err, core.ErrRecoveryRequired) || r.Groups[0].State != "failed" || !r.Groups[0].Result.Reset || len(f.deleted) != 1 {
		t.Fatal(r, err)
	}
}

func TestCacheCatalogBindsVisibleEnvironmentNamesAndOwnership(t *testing.T) {
	w, f := maintenanceTestWorkflow()
	h, err := w.CatalogHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.Groups[0].History.Name != "compiler" || len(h.Groups[0].Environments) != 1 {
		t.Fatal(h)
	}
	f.env.Name = "replacement"
	if _, err := w.MaintainCatalog(context.Background(), h.Revision, true); !errors.Is(err, core.ErrCapabilityStale) || f.resets != 0 {
		t.Fatal(err)
	}
}

type catalogRecoverFixture struct{ recovered []core.PersistentResourceRef }

func (f *catalogRecoverFixture) RecoverEnvironmentGeneration(_ context.Context, ref core.PersistentResourceRef) (core.ResourceGenerationPublication, error) {
	f.recovered = append(f.recovered, ref)
	return core.ResourceGenerationPublication{State: "retained"}, nil
}
func TestCacheCatalogRecoveryUsesExactRetainedCandidatesWithoutProducer(t *testing.T) {
	w, f := maintenanceTestWorkflow()
	f.env = core.Environment{}
	recovery := &catalogRecoverFixture{}
	w.Recoverer = recovery
	h, err := w.CatalogHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.MaintainCatalog(context.Background(), h.Revision, false)
	if err != nil || len(recovery.recovered) != 1 || recovery.recovered[0] != f.resources[0].Ref() || f.resets != 0 || len(f.deleted) != 0 || r.Groups[0].Recovery.Entries[0].State != "retained" {
		t.Fatal(r, err, recovery)
	}
}
