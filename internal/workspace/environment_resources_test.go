package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type environmentDataBackend struct {
	store     *state.EnvironmentJSONStore
	runtime   *fakeEnvironmentRuntime
	resources map[string]core.PersistentResource
	creates   int
	fail      string
}

func (b *environmentDataBackend) Plan(_ context.Context, _, owner string) (string, error) {
	return "pool/" + owner, nil
}
func (b *environmentDataBackend) Create(ctx context.Context, r core.PersistentResource) error {
	held, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || held != r {
		return errors.New("create before durable ownership")
	}
	b.resources[r.ID] = r
	if r.EnvironmentInstance != "" {
		b.creates++
		if b.fail == fmt.Sprintf("create-%d", b.creates) {
			return errors.New("provider create result unknown")
		}
	}
	return nil
}
func (b *environmentDataBackend) Verify(ctx context.Context, r core.PersistentResource) error {
	if r.EnvironmentInstance != "" && r.CopySource == (core.PersistentResourceRef{}) {
		held, err := b.store.GetPersistentResource(ctx, r.ID)
		if err != nil || held.State != "created" {
			return errors.New("verification before durable completion")
		}
		if b.fail == "verify" {
			return errors.New("verification failed")
		}
	}
	if b.resources[r.ID].Ref() != r.Ref() {
		return core.ErrNotFound
	}
	return nil
}
func (b *environmentDataBackend) Delete(ctx context.Context, r core.PersistentResource) error {
	held, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || held != r || r.State != "deleting" {
		return errors.New("delete without reservation")
	}
	if r.EnvironmentInstance != "" {
		lease, err := b.store.GetWorkspaceLease(ctx, "cache-env")
		if err != nil || !lease.RuntimeAbsent || lease.State != core.WorkspaceLeaseCleanupRequired {
			return errors.New("delete before runtime absence fence")
		}
		if b.runtime.deleteErr != nil {
			return errors.New("unsafe delete after failed runtime cleanup")
		}
	}
	if b.fail == "delete" {
		return errors.New("provider absence unknown")
	}
	delete(b.resources, r.ID)
	return nil
}
func (b *environmentDataBackend) Copy(ctx context.Context, source, target core.PersistentResource) error {
	held, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || held != target || held.CopySource != source.Ref() {
		return errors.New("copy before source reservation")
	}
	if b.resources[source.ID].Ref() != source.Ref() {
		return core.ErrNotFound
	}
	b.resources[target.ID] = target
	if b.fail == "copy" {
		return errors.New("copy outcome unknown")
	}
	return nil
}

type dataEnvironmentRuntime struct{ *fakeEnvironmentRuntime }

func (*dataEnvironmentRuntime) SupportsEnvironmentResources() bool { return true }

func dataEnvironmentService(t *testing.T, fail string) (*Service, *state.EnvironmentJSONStore, *environmentDataBackend, core.EnvironmentSpec, *persistentresource.Service) {
	t.Helper()
	ctx := context.Background()
	catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-cache-env"}}
	backend := &environmentDataBackend{store: catalog, runtime: runtime, resources: map[string]core.PersistentResource{}}
	resources := &persistentresource.Service{Store: catalog, Backend: backend}
	retained, err := resources.Create(ctx, "oci:retained", "oci-containerd")
	if err != nil {
		t.Fatal(err)
	}
	selected := []core.EnvironmentResourceSelection{}
	for _, name := range []string{"compiler", "packages"} {
		g, err := catalog.EnsureResourceGeneration(ctx, name, "build-cache", strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
		selected = append(selected, core.EnvironmentResourceSelection{Key: name, Target: "/home/dev/.cache/" + name, Origin: g})
	}
	svc := New(&dataEnvironmentRuntime{runtime}, catalog)
	svc.ConfigureEnvironmentResources(resources, func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		return selected, nil
	})
	backend.fail = fail
	return svc, catalog, backend, core.EnvironmentSpec{Name: "cache-env", WorkspacePath: t.TempDir(), PersistentResource: retained.ID}, resources
}

func TestEnvironmentDataOrdinaryLifecycleRetainsWorkspaceAndOCI(t *testing.T) {
	svc, catalog, b, spec, resources := dataEnvironmentService(t, "")
	ctx := context.Background()
	env, err := svc.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := catalog.GetWorkspaceLease(ctx, env.Name)
	if err != nil || !lease.MatchesEnvironment(env) || len(env.Attachments) != 2 {
		t.Fatal(lease, err)
	}
	if len(b.runtime.createSpec.Attachments) != 2 {
		t.Fatal("runtime did not receive all data")
	}
	for _, a := range env.Attachments {
		if err := resources.Delete(ctx, a.Resource.ID); !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal(err)
		}
	}
	if err := svc.Delete(ctx, env.Name); err != nil {
		t.Fatal(err)
	}
	if len(b.runtime.deleteRefs) != 1 || len(b.resources) != 1 || b.resources[spec.PersistentResource].ID == "" {
		t.Fatal("wrong cleanup scope", b.resources, b.runtime.deleteRefs)
	}
	retained, err := catalog.GetPersistentResource(ctx, spec.PersistentResource)
	if err != nil || retained.State != "ready" {
		t.Fatal(retained, err)
	}
	if _, err := svc.provider.Resolve(ctx, WorkspaceRequest{Path: spec.WorkspacePath}); err != nil {
		t.Fatal("Workspace removed", err)
	}
	if _, err := catalog.GetWorkspaceLease(ctx, env.Name); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestEnvironmentDataCreateFailureCleanup(t *testing.T) {
	for _, failure := range []string{"create-1", "create-2", "verify", "runtime", "runtime-unknown"} {
		t.Run(failure, func(t *testing.T) {
			svc, catalog, b, spec, _ := dataEnvironmentService(t, failure)
			ctx := context.Background()
			if failure == "runtime" {
				b.runtime.createErr = errors.New("runtime rejected before create")
			}
			if failure == "runtime-unknown" {
				b.runtime.createErr = core.ErrRecoveryRequired
			}
			if _, err := svc.Create(ctx, spec); err == nil {
				t.Fatal("expected failure")
			}
			all, err := catalog.ListPersistentResources(ctx)
			if err != nil {
				t.Fatal(err)
			}
			held, leaseErr := catalog.GetWorkspaceLease(ctx, spec.Name)
			switch failure {
			case "create-1", "create-2":
				if leaseErr != nil || !held.RuntimeAbsent || held.State != core.WorkspaceLeaseCleanupRequired || len(all) != 2 {
					t.Fatal(held, all, leaseErr)
				}
				if err := svc.Delete(ctx, spec.Name); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("unknown child creation was discarded", err)
				}
				if len(b.runtime.deleteRefs) != 0 {
					t.Fatal("no runtime was created")
				}
			case "runtime-unknown":
				if leaseErr != nil || held.RuntimeAbsent || len(all) != 3 {
					t.Fatal(held, all, leaseErr)
				}
				if err := svc.Delete(ctx, spec.Name); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(err)
				}
			default:
				if !errors.Is(leaseErr, core.ErrNotFound) || len(all) != 1 {
					t.Fatal(held, all, leaseErr)
				}
			}
		})
	}
}

func TestEnvironmentDataFailedRuntimeOrChildDeleteCanRetry(t *testing.T) {
	for _, failure := range []string{"runtime", "child"} {
		t.Run(failure, func(t *testing.T) {
			svc, catalog, b, spec, _ := dataEnvironmentService(t, "")
			ctx := context.Background()
			env, err := svc.Create(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			if failure == "runtime" {
				b.runtime.deleteErr = errors.New("runtime absence unknown")
			} else {
				b.fail = "delete"
			}
			if err = svc.Delete(ctx, env.Name); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
			lease, err := catalog.GetWorkspaceLease(ctx, env.Name)
			if err != nil || lease.RuntimeAbsent != (failure == "child") || len(b.resources) != 3 {
				t.Fatal(lease, b.resources, err)
			}
			b.runtime.deleteErr = nil
			b.fail = ""
			if err = svc.Delete(ctx, env.Name); err != nil {
				t.Fatal(err)
			}
			if len(b.resources) != 1 {
				t.Fatal(b.resources)
			}
			expectedDeletes := 2
			if failure == "child" {
				expectedDeletes = 1
			}
			if len(b.runtime.deleteRefs) != expectedDeletes {
				t.Fatal("reissued runtime deletion after durable absence", b.runtime.deleteRefs)
			}
		})
	}
}

func TestEnvironmentDataUnsupportedRuntimeNeverAllocates(t *testing.T) {
	svc, catalog, b, spec, _ := dataEnvironmentService(t, "")
	svc.runtime = b.runtime
	if _, err := svc.Create(context.Background(), spec); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if b.creates != 0 || len(b.resources) != 1 {
		t.Fatal("unsupported provider allocated data")
	}
	if _, err := catalog.GetWorkspaceLease(context.Background(), spec.Name); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestEnvironmentDataGenerationCopiesOutliveSources(t *testing.T) {
	svc, catalog, b, spec, resources := dataEnvironmentService(t, "")
	ctx := context.Background()
	selected, err := svc.selectEnvironmentResources(ctx, core.EnvironmentResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range selected {
		published, err := resources.PublishGeneration(ctx, selected[i].Origin, func(context.Context, core.PersistentResource) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		selected[i].Origin = published.Generation
	}
	svc.ConfigureEnvironmentResources(resources, func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		return selected, nil
	})
	env, err := svc.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	for i, area := range selected {
		if env.Attachments[i].Origin != area.Origin {
			t.Fatal("lost origin receipt")
		}
		if _, err = catalog.ResetResourceGeneration(ctx, area.Origin, area.Origin.Compatibility); err != nil {
			t.Fatal(err)
		}
		if err = resources.Delete(ctx, area.Origin.Current.ID); err != nil {
			t.Fatal(err)
		}
	}
	current, err := catalog.GetEnvironment(ctx, env.Name)
	if err != nil || !current.Equal(env) {
		t.Fatal("source deletion changed producer", err)
	}
	if err = svc.Delete(ctx, env.Name); err != nil {
		t.Fatal(err)
	}
	if len(b.resources) != 1 {
		t.Fatal("wrong retained scope", b.resources)
	}
}

func TestEnvironmentDataUnknownCopyKeepsSourceAndParent(t *testing.T) {
	svc, catalog, b, spec, resources := dataEnvironmentService(t, "")
	ctx := context.Background()
	selected, _ := svc.selectEnvironmentResources(ctx, core.EnvironmentResourceRequest{})
	published, err := resources.PublishGeneration(ctx, selected[0].Origin, func(context.Context, core.PersistentResource) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	selected[0].Origin = published.Generation
	svc.ConfigureEnvironmentResources(resources, func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		return selected, nil
	})
	b.fail = "copy"
	if _, err = svc.Create(ctx, spec); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	lease, err := catalog.GetWorkspaceLease(ctx, spec.Name)
	if err != nil || !lease.RuntimeAbsent {
		t.Fatal(lease, err)
	}
	target, err := catalog.GetPersistentResource(ctx, lease.Attachments[0].Resource.ID)
	if err != nil || target.State != "creating" || target.CopyCompleted {
		t.Fatal(target, err)
	}
	if _, err = catalog.ResetResourceGeneration(ctx, published.Generation, published.Generation.Compatibility); err != nil {
		t.Fatal(err)
	}
	if err = resources.Delete(ctx, published.Candidate.ID); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal(err)
	}
	if _, err = catalog.GetPersistentResource(ctx, lease.Attachments[1].Resource.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("unstarted sibling not cleaned", err)
	}
}

type failCompletedDataCommit struct {
	*state.EnvironmentJSONStore
	remaining int
}

func (s *failCompletedDataCommit) CommitPersistentResourceCreate(ctx context.Context, r core.PersistentResource) error {
	if r.EnvironmentInstance != "" && r.CopyCompleted && s.remaining > 0 {
		s.remaining--
		return errors.New("publication write failed after durable copy completion")
	}
	return s.EnvironmentJSONStore.CommitPersistentResourceCreate(ctx, r)
}
func TestEnvironmentDataCompletedCopyRecoveryUsesSharedReceipt(t *testing.T) {
	svc, catalog, b, spec, resources := dataEnvironmentService(t, "")
	ctx := context.Background()
	selected, _ := svc.selectEnvironmentResources(ctx, core.EnvironmentResourceRequest{})
	published, err := resources.PublishGeneration(ctx, selected[0].Origin, func(context.Context, core.PersistentResource) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	selected[0].Origin = published.Generation
	failing := &failCompletedDataCommit{EnvironmentJSONStore: catalog, remaining: 1}
	resources.Store = failing
	svc.ConfigureEnvironmentResources(resources, func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		return selected, nil
	})
	if _, err = svc.Create(ctx, spec); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("failed create misreported", err)
	}
	if failing.remaining != 0 {
		t.Fatal("did not reach completed-copy publication")
	}
	if _, err = catalog.GetWorkspaceLease(ctx, spec.Name); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("completed-copy recovery did not finish cleanup", err)
	}
	if len(b.resources) != 2 || b.resources[published.Candidate.ID].ID == "" || b.resources[spec.PersistentResource].ID == "" {
		t.Fatal("wrong retained data", b.resources)
	}
}
