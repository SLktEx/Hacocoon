package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeEnvironmentRuntime struct {
	createSpec    core.EnvironmentRuntimeSpec
	createResult  core.EnvironmentRuntime
	createErr     error
	execRef       string
	execRequest   core.ExecutionRequest
	execResult    core.ExecutionResult
	execErr       error
	shellRef      string
	shellErr      error
	deleteRefs    []string
	deleteErr     error
	cleanupCtxErr error
}

func (f *fakeEnvironmentRuntime) CreateEnvironment(_ context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	f.createSpec = spec
	return f.createResult, f.createErr
}

func (f *fakeEnvironmentRuntime) ExecEnvironment(_ context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	f.execRef = ref
	f.execRequest = req
	return f.execResult, f.execErr
}

func (f *fakeEnvironmentRuntime) ShellEnvironment(_ context.Context, ref string) error {
	f.shellRef = ref
	return f.shellErr
}

func (f *fakeEnvironmentRuntime) DeleteEnvironment(ctx context.Context, ref string) error {
	f.deleteRefs = append(f.deleteRefs, ref)
	f.cleanupCtxErr = ctx.Err()
	return f.deleteErr
}

type fakeEnvironmentStore struct {
	environments map[string]core.Environment
	getErr       error
	putErr       error
	deleteErr    error
	deleted      []string
}

func newFakeEnvironmentStore() *fakeEnvironmentStore {
	return &fakeEnvironmentStore{environments: map[string]core.Environment{}}
}

func (f *fakeEnvironmentStore) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	if f.getErr != nil {
		return core.Environment{}, f.getErr
	}
	environment, ok := f.environments[name]
	if !ok {
		return core.Environment{}, core.ErrNotFound
	}
	return environment, nil
}

func (f *fakeEnvironmentStore) PutEnvironment(_ context.Context, environment core.Environment) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.environments[environment.Name] = environment
	return nil
}

func (f *fakeEnvironmentStore) DeleteEnvironment(_ context.Context, name string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, name)
	delete(f.environments, name)
	return nil
}

func TestCreateResolvesWorkspaceAndPersistsEnvironment(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "workspace-link")
	if err := os.Symlink(workspaceDir, link); err != nil {
		t.Fatal(err)
	}

	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
	store := newFakeEnvironmentStore()
	service := New(runtime, store)
	fixed := time.Date(2026, 8, 29, 6, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }

	environment, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: link})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.createSpec.Name != "demo" || runtime.createSpec.WorkspacePath != workspaceDir {
		t.Fatalf("runtime spec = %#v", runtime.createSpec)
	}
	if environment.Name != "demo" || environment.RuntimeRef != "haco-demo" || environment.Workspace.Path != workspaceDir || !environment.CreatedAt.Equal(fixed) {
		t.Fatalf("environment = %#v", environment)
	}
	if got := store.environments["demo"]; !reflect.DeepEqual(got, environment) {
		t.Fatalf("stored = %#v, want %#v", got, environment)
	}
}

func TestCreateRejectsInvalidInputsBeforeRuntime(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []core.EnvironmentSpec{
		{Name: "Bad_Name", WorkspacePath: root},
		{Name: "demo", WorkspacePath: ""},
		{Name: "demo", WorkspacePath: filepath.Join(root, "missing")},
		{Name: "demo", WorkspacePath: file},
	}
	for _, spec := range cases {
		t.Run(spec.Name+spec.WorkspacePath, func(t *testing.T) {
			runtime := &fakeEnvironmentRuntime{}
			_, err := New(runtime, newFakeEnvironmentStore()).Create(context.Background(), spec)
			if err == nil {
				t.Fatal("expected error")
			}
			if runtime.createSpec != (core.EnvironmentRuntimeSpec{}) {
				t.Fatalf("runtime called with %#v", runtime.createSpec)
			}
		})
	}
}

func TestCreateRefusesExistingEnvironment(t *testing.T) {
	root := t.TempDir()
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}
	runtime := &fakeEnvironmentRuntime{}

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("error = %v", err)
	}
	if runtime.createSpec != (core.EnvironmentRuntimeSpec{}) {
		t.Fatalf("runtime called with %#v", runtime.createSpec)
	}
}

func TestCreateCleansRuntimeWhenPersistenceFailsEvenAfterCancellation(t *testing.T) {
	root := t.TempDir()
	persistErr := errors.New("disk full")
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
	store := newFakeEnvironmentStore()
	store.putErr = persistErr
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New(runtime, store).Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, persistErr) {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(runtime.deleteRefs, []string{"haco-demo"}) {
		t.Fatalf("cleanup refs = %#v", runtime.deleteRefs)
	}
	if runtime.cleanupCtxErr != nil {
		t.Fatalf("cleanup context remained canceled: %v", runtime.cleanupCtxErr)
	}
}

func TestExecForwardsEnvironmentAndPreservesResult(t *testing.T) {
	runtimeErr := errors.New("remote exit")
	runtime := &fakeEnvironmentRuntime{
		execResult: core.ExecutionResult{ExitCode: 7, Stdout: "out", Stderr: "err"},
		execErr:    runtimeErr,
	}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}
	service := New(runtime, store)

	request := core.ExecutionRequest{Argv: []string{"printf", "%s", "hello world"}}
	result, err := service.Exec(context.Background(), "demo", request)
	if !errors.Is(err, runtimeErr) || result.ExitCode != 7 || result.Stdout != "out" || result.Stderr != "err" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if runtime.execRef != "haco-demo" || !reflect.DeepEqual(runtime.execRequest, request) {
		t.Fatalf("forwarded ref=%q request=%#v", runtime.execRef, runtime.execRequest)
	}
}

func TestDeleteKeepsMetadataWhenRuntimeDeleteFails(t *testing.T) {
	deleteErr := errors.New("incus busy")
	runtime := &fakeEnvironmentRuntime{deleteErr: deleteErr}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}

	err := New(runtime, store).Delete(context.Background(), "demo")
	if !errors.Is(err, deleteErr) {
		t.Fatalf("error = %v", err)
	}
	if _, ok := store.environments["demo"]; !ok {
		t.Fatal("metadata removed after runtime delete failure")
	}
	if len(store.deleted) != 0 {
		t.Fatalf("store delete calls = %#v", store.deleted)
	}
}

func TestDeleteRemovesRuntimeBeforeMetadata(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}

	if err := New(runtime, store).Delete(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.deleteRefs, []string{"haco-demo"}) || !reflect.DeepEqual(store.deleted, []string{"demo"}) {
		t.Fatalf("runtime deletes=%#v metadata deletes=%#v", runtime.deleteRefs, store.deleted)
	}
}

func TestShellUsesStoredRuntimeReference(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}

	if err := New(runtime, store).Shell(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if runtime.shellRef != "haco-demo" {
		t.Fatalf("shell ref = %q", runtime.shellRef)
	}
}
