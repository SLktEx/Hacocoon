package agenthost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeBackend struct {
	environments map[string]core.Environment
	creates      int
	deletes      int
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{environments: map[string]core.Environment{}}
}

func (f *fakeBackend) Create(_ context.Context, spec core.EnvironmentSpec) (core.Environment, error) {
	if _, exists := f.environments[spec.Name]; exists {
		return core.Environment{}, core.ErrAlreadyExists
	}
	environment := core.Environment{
		Name: spec.Name,
		Workspace: core.Workspace{
			ID:   core.WorkspaceID("workspace:" + spec.WorkspacePath),
			Path: spec.WorkspacePath,
		},
		AccessMode: spec.AccessMode,
		RuntimeRef: "runtime:" + spec.Name,
		CreatedAt:  time.Unix(1, 0).UTC(),
	}
	f.environments[spec.Name] = environment
	f.creates++
	return environment, nil
}

func (f *fakeBackend) Delete(_ context.Context, name string) error {
	delete(f.environments, name)
	f.deletes++
	return nil
}

func (f *fakeBackend) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	environment, ok := f.environments[name]
	if !ok {
		return core.Environment{}, fmt.Errorf("environment %q: %w", name, core.ErrNotFound)
	}
	return environment, nil
}

func TestAcquireCreatesOneEnvironmentPerAgentSession(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)
	workspace := t.TempDir()

	first, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: workspace})
	if err != nil {
		t.Fatalf("Acquire first session: %v", err)
	}
	second, err := broker.Acquire(context.Background(), Spec{SessionID: "session-b", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatalf("Acquire second session: %v", err)
	}
	if first.EnvironmentName == second.EnvironmentName {
		t.Fatalf("different sessions shared environment %q", first.EnvironmentName)
	}
	if backend.creates != 2 {
		t.Fatalf("creates = %d, want 2", backend.creates)
	}
}

func TestAcquireIsIdempotentForExactBinding(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)
	workspace := t.TempDir()
	spec := Spec{SessionID: "session-a", WorkspacePath: workspace, AccessMode: core.WorkspaceReadWrite}

	first, err := broker.Acquire(context.Background(), spec)
	if err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	second, err := broker.Acquire(context.Background(), spec)
	if err != nil {
		t.Fatalf("Acquire second: %v", err)
	}
	if first != second {
		t.Fatalf("bindings differ: first=%+v second=%+v", first, second)
	}
	if backend.creates != 1 {
		t.Fatalf("creates = %d, want 1", backend.creates)
	}
}

func TestAcquireRejectsSessionRebinding(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)

	if _, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: t.TempDir()}); err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	_, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: t.TempDir()})
	if !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("Acquire rebinding error = %v, want ErrAlreadyExists", err)
	}
	if backend.creates != 1 {
		t.Fatalf("creates = %d, want 1", backend.creates)
	}
}

func TestAcquireKeepsAccessModeInBinding(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)
	workspace := t.TempDir()

	binding, err := broker.Acquire(context.Background(), Spec{
		SessionID:     "session-ro",
		WorkspacePath: workspace,
		AccessMode:    core.WorkspaceReadOnly,
	})
	if err != nil {
		t.Fatalf("Acquire read-only: %v", err)
	}
	if binding.AccessMode != core.WorkspaceReadOnly {
		t.Fatalf("access mode = %q, want %q", binding.AccessMode, core.WorkspaceReadOnly)
	}
}

func TestReleaseDeletesOnlySessionEnvironment(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)
	workspaceA := t.TempDir()
	workspaceB := t.TempDir()
	first, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: workspaceA})
	if err != nil {
		t.Fatal(err)
	}
	second, err := broker.Acquire(context.Background(), Spec{SessionID: "session-b", WorkspacePath: workspaceB})
	if err != nil {
		t.Fatal(err)
	}

	if err := broker.Release(context.Background(), "session-a"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, ok := backend.environments[first.EnvironmentName]; ok {
		t.Fatalf("released environment %q still exists", first.EnvironmentName)
	}
	if _, ok := backend.environments[second.EnvironmentName]; !ok {
		t.Fatalf("unrelated environment %q was deleted", second.EnvironmentName)
	}
}

func TestLookupReturnsExistingBinding(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)
	workspace := t.TempDir()
	created, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: workspace})
	if err != nil {
		t.Fatal(err)
	}

	lookedUp, err := broker.Lookup(context.Background(), "session-a")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if lookedUp != created {
		t.Fatalf("Lookup = %+v, want %+v", lookedUp, created)
	}
}

func TestAcquireCanonicalizesWorkspacePath(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)
	workspace := t.TempDir()
	child := filepath.Join(workspace, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	binding, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: filepath.Join(child, "..", "child")})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if binding.WorkspacePath != filepath.Clean(child) {
		t.Fatalf("workspace = %q, want %q", binding.WorkspacePath, filepath.Clean(child))
	}
}

func TestRejectsInvalidSessionID(t *testing.T) {
	backend := newFakeBackend()
	broker := New(backend, backend)
	workspace := t.TempDir()

	for _, sessionID := range []string{"", " session", "session\nother", string(make([]byte, maxSessionIDBytes+1))} {
		_, err := broker.Acquire(context.Background(), Spec{SessionID: sessionID, WorkspacePath: workspace})
		if !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("session id %q error = %v, want ErrInvalidArgument", sessionID, err)
		}
	}
}
