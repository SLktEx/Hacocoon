//go:build linux

package controlapi

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
)

type repositoryAPIBackend struct {
	mu        sync.Mutex
	volumes   map[string]gitrepo.Object
	connected []core.Environment
}

func (b *repositoryAPIBackend) Plan(_ context.Context, kind, id string) (string, error) {
	return "pool/" + kind + "-" + id, nil
}
func (b *repositoryAPIBackend) CreateVolume(_ context.Context, object gitrepo.Object, source *gitrepo.Object) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.volumes[object.NativeRef]; ok {
		return core.ErrAlreadyExists
	}
	if source != nil && b.volumes[source.NativeRef].Owner != source.Owner {
		return core.ErrCapabilityStale
	}
	b.volumes[object.NativeRef] = object
	return nil
}
func (b *repositoryAPIBackend) InspectVolume(_ context.Context, object gitrepo.Object) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.volumes[object.NativeRef].Owner != object.Owner {
		return core.ErrCapabilityStale
	}
	return nil
}
func (b *repositoryAPIBackend) Populate(ctx context.Context, object gitrepo.Object) error {
	return b.InspectVolume(ctx, object)
}
func (b *repositoryAPIBackend) RunGit(context.Context, gitadapter.AgentRequest) (gitadapter.Response, error) {
	return gitadapter.Response{}, core.ErrUnsupported
}
func (b *repositoryAPIBackend) ConnectGit(_ context.Context, env core.Environment, object gitrepo.Object, socket string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if env.Workspace.ID != core.WorkspaceID("workspace:managed:"+object.Owner) || filepath.Base(socket) != env.Name+".sock" {
		return core.ErrCapabilityStale
	}
	b.connected = append(b.connected, env)
	return nil
}

type repositoryAPIEnvironments struct {
	mu  sync.Mutex
	env core.Environment
}

func (s *repositoryAPIEnvironments) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name != s.env.Name {
		return core.Environment{}, core.ErrNotFound
	}
	return s.env, nil
}

func TestRepositoryWireOwnsCopies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := t.TempDir()
	backend := &repositoryAPIBackend{volumes: map[string]gitrepo.Object{}}
	service := gitrepo.NewRepositoryService(filepath.Join(root, "repositories"), backend)
	environments := &repositoryAPIEnvironments{}
	broker := gitrepo.NewBroker(service, environments, filepath.Join(root, "sockets"))
	if err := broker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterRepositories(s, service, broker); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]gitrepo.Object{}
	for _, id := range []string{"api", "web"} {
		request := RepositoryCloneRequest{ID: id, Remote: "https://github.com/example/" + id + ".git", Branch: "main"}
		object, err := client.CloneRepository(ctx, request)
		if err != nil || object.ID != id || object.Repository != id || object.Remote != request.Remote || object.Branch != request.Branch || object.Kind != "repo" || object.State != "ready" || len(object.Owner) != 32 {
			t.Fatal("clone receipt lost ownership/routing", object, err)
		}
		sources[id] = object
	}
	single, err := client.CopyWorkspace(ctx, WorkspaceCopyRequest{ID: "single", Repository: "api"})
	if err != nil || single.State != "ready" || single.Owner == sources["api"].Owner || single.NativeRef == sources["api"].NativeRef || single.Remote != sources["api"].Remote {
		t.Fatal("workspace copy retained source authority", single, err)
	}
	group, err := client.CopyWorkspace(ctx, WorkspaceCopyRequest{ID: "group", Repositories: []string{"api", "web"}})
	if err != nil || group.State != "ready" || len(group.Members) != 2 || group.Repository != "" {
		t.Fatal("collection receipt is incomplete", group, err)
	}
	for _, member := range group.Members {
		source := sources[member.Repository]
		if member.Owner == source.Owner || member.NativeRef == source.NativeRef || member.State != "ready" || member.Remote != source.Remote {
			t.Fatal("collection member lost independent owner", member)
		}
	}
	for _, object := range []gitrepo.Object{sources["api"], sources["web"], single, group} {
		durable, err := gitrepo.NewRepositoryService(service.Root, backend).Get(object.Kind, object.ID)
		if err != nil || !reflect.DeepEqual(durable, object) {
			t.Fatal("wire receipt differs from durable catalog", durable, err)
		}
	}
	environments.mu.Lock()
	environments.env = core.Environment{Name: "dev", RuntimeRef: "incus:owned", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + single.Owner), Path: "managed:" + single.ID}}
	environments.mu.Unlock()
	if err := client.ConnectGit(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if pending, err := client.PendingGit(ctx); err != nil || len(pending) != 0 {
		t.Fatal("connect created a guest Git approval", pending, err)
	}
	if err := client.DecideGit(ctx, "missing", true); err == nil {
		t.Fatal("unknown approval accepted")
	}
	listed, err := client.RepositoryManage(ctx, RepositoryManageRequest{Operation: "list"})
	if err != nil || len(listed.Sources) != 2 {
		t.Fatal(listed, err)
	}
	wantWorkspaces := map[string][]string{"api": {"group", "single"}, "web": {"group"}}
	for _, use := range listed.Sources {
		if !reflect.DeepEqual(use.Source, sources[use.Source.ID]) || !reflect.DeepEqual(use.Workspaces, wantWorkspaces[use.Source.ID]) {
			t.Fatal("repository review lost owner or dependent Workspaces", use)
		}
	}
	for _, request := range []WorkspaceCopyRequest{{ID: "bad", Repository: "api", Repositories: []string{"api", "web"}}, {ID: "bad", Repositories: []string{"api", "api"}}, {ID: "bad", Repository: "missing"}} {
		if _, err := client.CopyWorkspace(ctx, request); err == nil {
			t.Fatal("ambiguous/missing workspace source accepted", request)
		}
	}
	for _, method := range []string{MethodRepositoryClone, MethodWorkspaceCopy, MethodGitConnect, MethodGitDecide} {
		var status *control.StatusError
		if err := client.wire.Call(ctx, method, "invalid request", nil); !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatal("malformed repository request accepted", method, err)
		}
	}
	if _, err := client.CloneRepository(ctx, RepositoryCloneRequest{ID: "bad", Remote: "https://token@github.com/example/api.git", Branch: "main"}); err == nil {
		t.Fatal("credential-bearing remote accepted")
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if len(backend.volumes) != 5 || len(backend.connected) != 1 || backend.connected[0].Name != "dev" {
		t.Fatal("invalid request allocated native volume or Git connection", backend.volumes, backend.connected)
	}
}
