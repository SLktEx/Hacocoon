package persistentresource_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
)

type generationBackend struct {
	store          *state.EnvironmentJSONStore
	mu             sync.Mutex
	resources      map[string]core.PersistentResource
	plans, deletes int
	failDelete     bool
}

func (b *generationBackend) Plan(_ context.Context, _ string, owner string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.plans++
	return "pool/" + owner, nil
}
func (b *generationBackend) Create(ctx context.Context, r core.PersistentResource) error {
	saved, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || saved != r {
		return errors.New("create before durable ownership")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.resources[r.ID] = r
	return nil
}
func (b *generationBackend) Verify(_ context.Context, r core.PersistentResource) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if saved, ok := b.resources[r.ID]; !ok || saved.Ref() != r.Ref() {
		return errors.New("missing owned resource")
	}
	return nil
}
func (b *generationBackend) Delete(ctx context.Context, r core.PersistentResource) error {
	saved, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || saved != r || r.State != "deleting" {
		return errors.New("delete before reservation")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deletes++
	if b.failDelete {
		return errors.New("absence uncertain")
	}
	if owned, ok := b.resources[r.ID]; ok && owned.Ref() != r.Ref() {
		return errors.New("wrong owner")
	}
	delete(b.resources, r.ID)
	return nil
}
func generationService(t *testing.T) (*persistentresource.Service, *generationBackend, core.ResourceGeneration) {
	t.Helper()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	b := &generationBackend{store: store, resources: map[string]core.PersistentResource{}}
	g, err := store.EnsureResourceGeneration(context.Background(), "compiler", "build-cache", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	return &persistentresource.Service{Store: store, Backend: b}, b, g
}
func preparedGeneration(context.Context, core.PersistentResource) error { return nil }

func TestGenerationPublicationConcurrentProducerKeepsOneWholeSource(t *testing.T) {
	for _, failDelete := range []bool{false, true} {
		t.Run(map[bool]string{false: "cleanup", true: "uncertain-cleanup"}[failDelete], func(t *testing.T) {
			svc, b, g := generationService(t)
			b.failDelete = failDelete
			ready := make(chan struct{}, 2)
			release := make(chan struct{})
			results := make([]core.ResourceGenerationPublication, 2)
			errs := make([]error, 2)
			var wg sync.WaitGroup
			for i := range results {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					results[i], errs[i] = svc.PublishGeneration(context.Background(), g, func(ctx context.Context, r core.PersistentResource) error { ready <- struct{}{}; <-release; return nil })
				}(i)
			}
			<-ready
			<-ready
			close(release)
			wg.Wait()
			published, skipped := 0, 0
			for i, result := range results {
				switch result.State {
				case "published":
					published++
					if errs[i] != nil || result.Generation.Number != 1 {
						t.Fatal(result, errs[i])
					}
				case "skipped":
					skipped++
					if failDelete || errs[i] != nil || result.Candidate.ID != "" {
						t.Fatal(result, errs[i])
					}
				case "cleanup-required":
					skipped++
					if !failDelete || !errors.Is(errs[i], core.ErrRecoveryRequired) || result.Candidate.ID == "" {
						t.Fatal(result, errs[i])
					}
				default:
					t.Fatal(result, errs[i])
				}
			}
			if published != 1 || skipped != 1 || b.plans != 2 || b.deletes != 1 {
				t.Fatal(results, errs, b.plans, b.deletes)
			}
			catalog, err := b.store.ListPersistentResources(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if failDelete {
				want = 2
			}
			if len(catalog) != want || len(b.resources) != want {
				t.Fatal(catalog, b.resources)
			}
			current, err := b.store.GetResourceGeneration(context.Background(), g.Name)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range catalog {
				if r.Ref() == current.Current {
					if r.State != "ready" {
						t.Fatal(r)
					}
				} else if r.State != "deleting" {
					t.Fatal(r)
				}
			}
		})
	}
}

func TestGenerationPublicationStaleAndPreparationFailure(t *testing.T) {
	svc, b, g := generationService(t)
	ctx := context.Background()
	good, err := svc.PublishGeneration(ctx, g, preparedGeneration)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := svc.PublishGeneration(ctx, g, func(context.Context, core.PersistentResource) error { t.Error("stale producer prepared"); return nil })
	if err != nil || stale.State != "skipped" || stale.Generation != good.Generation || b.plans != 1 {
		t.Fatal(stale, err, b.plans)
	}
	failure := errors.New("copy incomplete")
	failed, err := svc.PublishGeneration(ctx, good.Generation, func(context.Context, core.PersistentResource) error { return failure })
	if !errors.Is(err, failure) || !errors.Is(err, core.ErrRecoveryRequired) || failed.State != "recovery-required" {
		t.Fatal(failed, err)
	}
	current, err := b.store.GetResourceGeneration(ctx, g.Name)
	if err != nil || current != good.Generation {
		t.Fatal(current, err)
	}
	held, err := b.store.GetPersistentResource(ctx, failed.Candidate.ID)
	if err != nil || held.State != "creating" {
		t.Fatal(held, err)
	}
	if err := svc.Delete(ctx, held.ID); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	if b.deletes != 0 || len(b.resources) != 2 {
		t.Fatal(b.deletes, b.resources)
	}
}

type uncertainGenerationStore struct {
	*state.EnvironmentJSONStore
	afterWrite bool
}

func (s uncertainGenerationStore) AdvanceResourceGeneration(ctx context.Context, g core.ResourceGeneration, r core.PersistentResourceRef) (core.ResourceGeneration, error) {
	if s.afterWrite {
		if _, err := s.EnvironmentJSONStore.AdvanceResourceGeneration(ctx, g, r); err != nil {
			return core.ResourceGeneration{}, err
		}
	}
	return core.ResourceGeneration{}, errors.New("selection persistence uncertain")
}
func TestGenerationPublicationUncertainSelectionNeverDeletesCandidate(t *testing.T) {
	for _, afterWrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-write", true: "after-write"}[afterWrite], func(t *testing.T) {
			svc, b, g := generationService(t)
			svc.Store = uncertainGenerationStore{EnvironmentJSONStore: b.store, afterWrite: afterWrite}
			result, err := svc.PublishGeneration(context.Background(), g, preparedGeneration)
			if !errors.Is(err, core.ErrRecoveryRequired) || result.State != "recovery-required" || result.Candidate.ID == "" || b.deletes != 0 {
				t.Fatal(result, err, b.deletes)
			}
			held, err := b.store.GetPersistentResource(context.Background(), result.Candidate.ID)
			if err != nil || held.State != "ready" || len(b.resources) != 1 {
				t.Fatal(held, err)
			}
			current, err := b.store.GetResourceGeneration(context.Background(), g.Name)
			if err != nil {
				t.Fatal(err)
			}
			if afterWrite && current.Current != held.Ref() || !afterWrite && current != g {
				t.Fatal(current)
			}
		})
	}
}
