package agenthost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func newTestBroker(t *testing.T, backend *fakeBackend) (*Broker, string) {
	t.Helper()
	statePath := filepath.Join(t.TempDir(), "agent-bindings.json")
	return New(backend, backend, NewJSONBindingStore(statePath)), statePath
}

func TestAcquireCreatesDedicatedEnvironmentsAndIsIdempotent(t *testing.T) {
	backend := newFakeBackend()
	broker, _ := newTestBroker(t, backend)
	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	first, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: workspaceA})
	if err != nil {
		t.Fatal(err)
	}
	again, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: workspaceA})
	if err != nil {
		t.Fatal(err)
	}
	second, err := broker.Acquire(context.Background(), Spec{SessionID: "session-b", WorkspacePath: workspaceB})
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatalf("idempotent acquire changed binding: first=%+v again=%+v", first, again)
	}
	if first.EnvironmentName == second.EnvironmentName {
		t.Fatalf("distinct sessions shared environment %q", first.EnvironmentName)
	}
	if backend.creates != 2 {
		t.Fatalf("creates=%d, want 2", backend.creates)
	}
}

func TestAcquireRejectsWorkspaceAndModeRebinding(t *testing.T) {
	backend := newFakeBackend()
	broker, _ := newTestBroker(t, backend)
	workspace := t.TempDir()

	if _, err := broker.Acquire(context.Background(), Spec{
		SessionID:     "session-a",
		WorkspacePath: workspace,
		AccessMode:    core.WorkspaceReadOnly,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Acquire(context.Background(), Spec{
		SessionID:     "session-a",
		WorkspacePath: t.TempDir(),
		AccessMode:    core.WorkspaceReadOnly,
	}); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("workspace rebind error=%v, want ErrAlreadyExists", err)
	}
	if _, err := broker.Acquire(context.Background(), Spec{
		SessionID:     "session-a",
		WorkspacePath: workspace,
		AccessMode:    core.WorkspaceReadWrite,
	}); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("mode rebind error=%v, want ErrAlreadyExists", err)
	}
}

func TestReleaseRequiresPersistedBindingProof(t *testing.T) {
	backend := newFakeBackend()
	broker, _ := newTestBroker(t, backend)
	sessionID := "unbound-session"
	name := environmentName(sessionID)
	backend.environments[name] = core.Environment{
		Name:       name,
		Workspace:  core.Workspace{ID: "human", Path: t.TempDir()},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "runtime:human",
	}

	err := broker.Release(context.Background(), sessionID)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("release error=%v, want ErrNotFound", err)
	}
	if _, ok := backend.environments[name]; !ok {
		t.Fatalf("unbound environment %q was deleted", name)
	}
	if backend.deletes != 0 {
		t.Fatalf("delete calls=%d, want 0", backend.deletes)
	}
}

func TestReleaseDeletesOnlyBoundEnvironment(t *testing.T) {
	backend := newFakeBackend()
	broker, _ := newTestBroker(t, backend)
	first, err := broker.Acquire(context.Background(), Spec{SessionID: "session-a", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	second, err := broker.Acquire(context.Background(), Spec{SessionID: "session-b", WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	if err := broker.Release(context.Background(), "session-a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.environments[first.EnvironmentName]; ok {
		t.Fatalf("released environment still exists: %q", first.EnvironmentName)
	}
	if _, ok := backend.environments[second.EnvironmentName]; !ok {
		t.Fatalf("unrelated environment was deleted: %q", second.EnvironmentName)
	}
	if _, err := broker.Lookup(context.Background(), "session-a"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("released binding lookup error=%v, want ErrNotFound", err)
	}
}

func TestBindingSurvivesBrokerRestartWithoutRawSessionID(t *testing.T) {
	backend := newFakeBackend()
	broker, statePath := newTestBroker(t, backend)
	sessionID := "copilot:/sensitive-session-name"
	workspace := t.TempDir()
	created, err := broker.Acquire(context.Background(), Spec{SessionID: sessionID, WorkspacePath: workspace})
	if err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), sessionID) || strings.Contains(created.EnvironmentName, sessionID) {
		t.Fatalf("raw session id leaked into persisted identity")
	}

	restarted := New(backend, backend, NewJSONBindingStore(statePath))
	lookedUp, err := restarted.Lookup(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if lookedUp != created {
		t.Fatalf("restart lookup=%+v, want %+v", lookedUp, created)
	}
}

func TestAcquireRefusesUnboundDeterministicCollision(t *testing.T) {
	backend := newFakeBackend()
	broker, _ := newTestBroker(t, backend)
	sessionID := "collision"
	workspace := t.TempDir()
	name := environmentName(sessionID)
	backend.environments[name] = core.Environment{
		Name:       name,
		Workspace:  core.Workspace{ID: core.WorkspaceID("workspace:" + workspace), Path: workspace},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "runtime:preexisting",
	}

	_, err := broker.Acquire(context.Background(), Spec{SessionID: sessionID, WorkspacePath: workspace})
	if !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("acquire collision error=%v, want ErrAlreadyExists", err)
	}
}

func TestRejectsInvalidSessionID(t *testing.T) {
	backend := newFakeBackend()
	broker, _ := newTestBroker(t, backend)
	workspace := t.TempDir()
	for _, sessionID := range []string{"", " leading", "line\nbreak", string(make([]byte, maxSessionIDBytes+1))} {
		if _, err := broker.Acquire(context.Background(), Spec{SessionID: sessionID, WorkspacePath: workspace}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("session %q error=%v, want ErrInvalidArgument", sessionID, err)
		}
	}
}
