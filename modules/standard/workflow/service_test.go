package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type reposFixture struct {
	object       gitrepo.Object
	restores     int
	createCalls  int
	dataFail     error
	catalogReady bool
}

func (r *reposFixture) Get(kind, id string) (gitrepo.Object, error) {
	if id != r.object.ID {
		return gitrepo.Object{}, core.ErrNotFound
	}
	if r.object.State != "ready" {
		return r.object, core.ErrRecoveryRequired
	}
	return r.object, nil
}
func (r *reposFixture) CopyWorkspace(_ context.Context, id, repo string) (gitrepo.Object, error) {
	r.createCalls++
	r.object = gitrepo.Object{Kind: "work", ID: id, Owner: "new", State: "ready", Repository: repo}
	return r.object, nil
}
func (r *reposFixture) CopyWorkspaceSet(ctx context.Context, id string, repos []string) (gitrepo.Object, error) {
	object, err := r.CopyWorkspace(ctx, id, repos[0])
	object.Members = nil
	for _, name := range repos {
		object.Members = append(object.Members, gitrepo.Object{Repository: name})
	}
	r.object = object
	return object, err
}
func (r *reposFixture) RestoreWorkspaceWithData(ctx context.Context, id string, _ core.Snapshot, prepare func(context.Context, core.Workspace) error) (gitrepo.Object, error) {
	r.restores++
	object := gitrepo.Object{Kind: "work", ID: id, Owner: "fork", State: "creating"}
	err := prepare(ctx, core.Workspace{ID: "workspace:managed:fork"})
	if err == nil {
		object.State = "ready"
		r.catalogReady = true
	}
	return object, err
}

type envFixture struct {
	mu                                                 sync.Mutex
	envs                                               []core.Environment
	createCalls, startCalls, captureCalls, deleteCalls int
	failCreate, failCapture, failDelete                error
	captured                                           core.WorkspaceID
	saved                                              core.Snapshot
}

func (e *envFixture) List(context.Context) ([]core.Environment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]core.Environment(nil), e.envs...), nil
}
func (e *envFixture) ListManagedWorkspaces(context.Context) ([]workspaceapp.ManagedWorkspace, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	all := []workspaceapp.ManagedWorkspace{}
	for _, v := range e.envs {
		all = append(all, workspaceapp.ManagedWorkspace{Workspace: v.Workspace, Environments: []string{v.Name}})
	}
	return all, nil
}
func (e *envFixture) Create(_ context.Context, s core.EnvironmentSpec) (core.Environment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s.ExpectedWorkspace != "workspace:managed:owner" {
		return core.Environment{}, core.ErrCapabilityStale
	}
	if len(e.envs) > 0 {
		return core.Environment{}, core.ErrAlreadyExists
	}
	if e.failCreate != nil {
		return core.Environment{}, e.failCreate
	}
	e.createCalls++
	v := core.Environment{Name: s.Name, Workspace: core.Workspace{ID: s.ExpectedWorkspace}, AccessMode: core.WorkspaceReadWrite, Base: &core.BaseRef{Name: s.Base}}
	if !s.SkipDefaultResource {
		v.PersistentResource = core.PersistentResourceRef{ID: "oci:retained", Owner: "owner"}
	}
	e.envs = append(e.envs, v)
	return v, nil
}
func (e *envFixture) StartForWorkspace(_ context.Context, name string, id core.WorkspaceID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, v := range e.envs {
		if v.Name == name && v.Workspace.ID == id {
			e.startCalls++
			return nil
		}
	}
	return core.ErrCapabilityStale
}
func (e *envFixture) CaptureStoppedSnapshotForWorkspace(_ context.Context, _ string, id core.WorkspaceID) (core.Snapshot, error) {
	e.captureCalls++
	e.captured = id
	return e.saved, e.failCapture
}
func (e *envFixture) DeleteSnapshot(context.Context, string) error {
	e.deleteCalls++
	return e.failDelete
}

type storesFixture struct {
	work  core.WorkspaceID
	fail  error
	calls int
}

func (s *storesFixture) RestoreSnapshot(_ context.Context, id string, _ core.Snapshot, work core.WorkspaceID) (core.PersistentResource, error) {
	s.calls++
	s.work = work
	return core.PersistentResource{ID: id, Owner: "fork-store", WorkspaceID: work}, s.fail
}
func fixture() (*Service, *reposFixture, *envFixture) {
	r := &reposFixture{object: gitrepo.Object{Kind: "work", ID: "task", Owner: "owner", State: "ready", Repository: "one"}}
	e := &envFixture{}
	return &Service{Repositories: r, Environments: e}, r, e
}
func TestOpenConcurrentResumeAndRecreatePreservesWork(t *testing.T) {
	s, _, e := fixture()
	ctx := context.Background()
	spec := OpenSpec{Reference: Reference{Name: "task", Workspace: "workspace:managed:owner"}, OCI: "none", Base: "haco/ubuntu-24.04"}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.Open(ctx, spec)
			if err != nil || r.Workspace != spec.Workspace {
				t.Errorf("%+v %v", r, err)
			}
		}()
	}
	wg.Wait()
	if e.createCalls != 1 || len(e.envs) != 1 {
		t.Fatal("duplicate Env", e.createCalls)
	}
	if _, err := s.Open(ctx, OpenSpec{Reference: spec.Reference, Base: "haco/ubuntu-26.04"}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("implicit Base replacement", err)
	}
	e.envs = nil
	spec.Base = "haco/ubuntu-26.04"
	result, err := s.Open(ctx, spec)
	if err != nil || !result.Created || e.createCalls != 2 || result.Environment.PersistentResource.ID != "" {
		t.Fatal(result, err)
	}
}
func TestOpenRefusesRecycledReferenceAndIncompleteCreate(t *testing.T) {
	s, _, e := fixture()
	_, err := s.Open(context.Background(), OpenSpec{Reference: Reference{Name: "task", Workspace: "workspace:managed:old"}})
	if !errors.Is(err, core.ErrCapabilityStale) || e.createCalls != 0 {
		t.Fatal(err)
	}
	e.failCreate = core.ErrRecoveryRequired
	for i := 0; i < 2; i++ {
		_, err = s.Open(context.Background(), OpenSpec{Reference: Reference{Name: "task", Workspace: "workspace:managed:owner"}})
		if !errors.Is(err, core.ErrRecoveryRequired) || e.startCalls != 0 {
			t.Fatal(err)
		}
	}
}
func TestPrepareReusesExactMembershipAndRefusesPartialCopy(t *testing.T) {
	s, r, _ := fixture()
	r.object = gitrepo.Object{}
	spec := PrepareSpec{Name: "task", Repositories: []string{"one", "two"}}
	first, err := s.Prepare(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Repositories = []string{"two", "one"}
	next, err := s.Prepare(context.Background(), spec)
	if err != nil || next != first || r.createCalls != 1 {
		t.Fatal(next, err)
	}
	spec.Repositories = []string{"one"}
	if _, err = s.Prepare(context.Background(), spec); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatal(err)
	}
	r.object.State = "creating"
	if _, err = s.Prepare(context.Background(), spec); !errors.Is(err, core.ErrRecoveryRequired) || r.createCalls != 1 {
		t.Fatal(err)
	}
}
func TestForkCopiesStoppedAggregateWithIndependentStoreAndRetainsFailure(t *testing.T) {
	for _, mode := range []string{"oci", "none", "store-failure", "capture-failure", "cleanup-failure"} {
		t.Run(mode, func(t *testing.T) {
			s, r, e := fixture()
			stores := &storesFixture{}
			s.Stores = stores
			e.envs = []core.Environment{{Name: "source-env", Workspace: core.Workspace{ID: "workspace:managed:owner"}}}
			e.saved = core.Snapshot{ID: "snapshot", Source: core.SnapshotSource{Environment: core.Environment{PersistentResource: core.PersistentResourceRef{ID: "oci:source"}, Base: &core.BaseRef{Name: "haco/ubuntu-24.04"}}}}
			switch mode {
			case "none":
				e.saved.Source.Environment.PersistentResource = core.PersistentResourceRef{}
			case "store-failure":
				stores.fail = core.ErrRecoveryRequired
			case "capture-failure":
				e.failCapture = core.ErrIncompatibleState
			case "cleanup-failure":
				e.failDelete = core.ErrRecoveryRequired
			}
			result, err := s.Fork(context.Background(), Reference{Name: "task", Workspace: "workspace:managed:owner"}, "branch")
			if e.captured != "workspace:managed:owner" || e.deleteCalls != 1 || e.startCalls != 0 {
				t.Fatal("incorrect source lifecycle")
			}
			if mode == "capture-failure" {
				if err == nil || r.restores != 0 {
					t.Fatal(result, err)
				}
				return
			}
			if result.Workspace != "workspace:managed:fork" {
				t.Fatal(result)
			}
			if mode == "none" {
				if stores.calls != 0 || result.OCI != "none" {
					t.Fatal(result)
				}
			}
			if mode == "store-failure" {
				if err == nil || r.catalogReady || result.State != "recovery-required" {
					t.Fatal(result, err)
				}
				return
			}
			if mode == "cleanup-failure" {
				if !errors.Is(err, core.ErrRecoveryRequired) || result.TemporarySnapshot == "" {
					t.Fatal(result, err)
				}
				return
			}
			if err != nil || result.State != "ready" || result.TemporarySnapshot != "" {
				t.Fatal(result, err)
			}
			if mode == "oci" && (stores.work != result.Workspace || result.OCI != "oci:branch") {
				t.Fatal(result)
			}
		})
	}
}

func TestOpenNeverAdoptsRecycledStoreOwner(t *testing.T) {
	s, _, e := fixture()
	e.envs = []core.Environment{{Name: "env", Workspace: core.Workspace{ID: "workspace:managed:owner"}, AccessMode: core.WorkspaceReadWrite, PersistentResource: core.PersistentResourceRef{ID: "oci:retained", Owner: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}}
	spec := OpenSpec{Reference: Reference{Name: "task", Workspace: "workspace:managed:owner"}, OCI: "auto", ExpectedResource: core.PersistentResourceRef{ID: "oci:retained", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	_, err := s.Open(context.Background(), spec)
	if !errors.Is(err, core.ErrCapabilityStale) || e.startCalls != 0 || e.createCalls != 0 {
		t.Fatal("adopted recycled Store", err)
	}
}
