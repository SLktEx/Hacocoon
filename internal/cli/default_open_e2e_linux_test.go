//go:build linux

package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

// Native Git, volumes and runtime are fixtures. The shipped CLI process, Unix
// RPC, repository catalog, collection ownership and lifecycle/lease store are real.
// This is repository E2E coverage, not installed Incus/desktop acceptance.
func TestDefaultWorkflowE2E(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "haco")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/haco")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	native := &defaultNativeFixture{root: filepath.Join(root, "volumes"), volumes: map[string]gitrepo.Object{}}
	repos := gitrepo.NewRepositoryService(filepath.Join(root, "repos"), native)
	native.repos = repos
	catalog := state.NewEnvironmentJSONStore(filepath.Join(root, "state.json"))
	envs := workspaceapp.NewWithProvider(native, catalog, native)
	server := control.NewServer()
	if err := controlapi.RegisterRepositories(server, repos, nil); err != nil {
		t.Fatal(err)
	}
	if err := controlapi.RegisterWorkflow(server, &workflow.Service{Repositories: repos, Environments: envs}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(root, "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() { cancel(); <-done }()
	t.Setenv("HOME", home)
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	t.Setenv("HACO_UI_LANGUAGE", "en")
	runCLI := func(args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(ctx, bin, args...)
		var diagnostic strings.Builder
		cmd.Stderr = &diagnostic
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, &diagnostic)
		}
		if args[0] == "open" && !strings.Contains(diagnostic.String(), "Preparing project files") {
			t.Fatal("missing progress")
		}
		return out
	}
	runCLI("repo", "add", "api", "https://github.com/example/api.git")
	runCLI("repo", "add", "web", "https://github.com/example/web.git")
	open := func() workflow.OpenResult {
		t.Helper()
		var result workflow.OpenResult
		if err := json.Unmarshal(runCLI("open", "--client", "none", "--json"), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := open()
	if !first.Created || first.Environment.Base == nil || first.Environment.Base.Name != "fixture-default" {
		t.Fatal("default Base not selected", first)
	}
	work, err := repos.Get("work", first.Name)
	if err != nil || len(work.Members) != 2 {
		t.Fatal("collection absent", work, err)
	}
	for _, member := range work.Members {
		source, err := repos.Get("repo", member.Repository)
		if err != nil || source.Owner == member.Owner || source.NativeRef == member.NativeRef {
			t.Fatal("source reused as work", err)
		}
		if source.Branch != "" || member.Branch != "main" {
			t.Fatal("branch selection is not confined to the Workspace", source.Branch, member.Branch)
		}
		if err := os.WriteFile(filepath.Join(member.NativeRef, "edit"), []byte("uncommitted"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := envs.Stop(ctx, first.Environment.Name); err != nil {
		t.Fatal(err)
	}
	second := open()
	if second.Created || second.Reference != first.Reference || second.Environment.RuntimeRef != first.Environment.RuntimeRef {
		t.Fatal("resume replaced work", second)
	}
	for _, member := range work.Members {
		raw, err := os.ReadFile(filepath.Join(member.NativeRef, "edit"))
		if err != nil || string(raw) != "uncommitted" {
			t.Fatal("edit lost", err)
		}
	}
	native.mu.Lock()
	defer native.mu.Unlock()
	if native.creates != 1 || native.starts != 1 || !native.running {
		t.Fatal("reopen did not reuse lifecycle", native.creates, native.starts)
	}
	if native.resolves != 2 {
		t.Fatal("branches must be resolved once per source, not again on reopen", native.resolves)
	}
}

type defaultNativeFixture struct {
	mu              sync.Mutex
	root            string
	repos           *gitrepo.RepositoryService
	volumes         map[string]gitrepo.Object
	creates, starts int
	resolves        int
	running         bool
}

func (f *defaultNativeFixture) Plan(_ context.Context, kind, id string) (string, error) {
	return filepath.Join(f.root, kind+"-"+id), nil
}
func (f *defaultNativeFixture) CreateVolume(_ context.Context, object gitrepo.Object, source *gitrepo.Object) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.volumes[object.NativeRef]; ok {
		return core.ErrAlreadyExists
	}
	if source != nil && f.volumes[source.NativeRef].Owner != source.Owner {
		return core.ErrCapabilityStale
	}
	if err := os.MkdirAll(object.NativeRef, 0700); err != nil {
		return err
	}
	f.volumes[object.NativeRef] = object
	return nil
}
func (f *defaultNativeFixture) InspectVolume(_ context.Context, object gitrepo.Object) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.volumes[object.NativeRef].Owner != object.Owner {
		return core.ErrCapabilityStale
	}
	return nil
}
func (f *defaultNativeFixture) Populate(ctx context.Context, object gitrepo.Object) error {
	return f.InspectVolume(ctx, object)
}
func (f *defaultNativeFixture) RunGit(ctx context.Context, req gitadapter.AgentRequest) (gitadapter.Response, error) {
	if err := ctx.Err(); err != nil {
		return gitadapter.Response{}, err
	}
	if req.Operation != "resolve" {
		return gitadapter.Response{}, core.ErrUnsupported
	}
	if !gitadapter.ValidID(req.Repository) || (req.Branch != "" && req.Branch != "main") {
		return gitadapter.Response{}, core.ErrInvalidArgument
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	source, ok := f.volumes[filepath.Join(f.root, "repo-"+req.Repository)]
	if !ok || source.Kind != "repo" || source.Repository != req.Repository || source.Remote != req.Remote {
		return gitadapter.Response{}, core.ErrCapabilityStale
	}
	f.resolves++
	return gitadapter.Response{Ref: "refs/heads/main", OID: strings.Repeat("a", 40)}, nil
}
func (*defaultNativeFixture) ConnectGit(context.Context, core.Environment, gitrepo.Object, string) error {
	return core.ErrUnsupported
}
func (f *defaultNativeFixture) Resolve(_ context.Context, req workspaceapp.WorkspaceRequest) (core.Workspace, error) {
	object, err := f.repos.Get("work", strings.TrimPrefix(req.Path, "managed:"))
	return core.Workspace{ID: core.WorkspaceID("workspace:managed:" + object.Owner), Path: req.Path}, err
}
func (f *defaultNativeFixture) CreateEnvironment(_ context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	f.running = true
	return core.EnvironmentRuntime{Ref: "fixture-runtime", Base: &core.BaseRef{Name: "fixture-default"}, Resources: spec.Resources}, nil
}
func (f *defaultNativeFixture) StartEnvironment(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	f.running = true
	return nil
}
func (f *defaultNativeFixture) StopEnvironment(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = false
	return nil
}
func (*defaultNativeFixture) DeleteEnvironment(context.Context, string) error {
	return core.ErrUnsupported
}
func (*defaultNativeFixture) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, core.ErrUnsupported
}
func (*defaultNativeFixture) ShellEnvironment(context.Context, string) error {
	return core.ErrUnsupported
}
