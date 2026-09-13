package gitrepo

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type brokerStatusBackend struct {
	connected int
	gitCalls  int
}

func (*brokerStatusBackend) Plan(context.Context, string, string) (string, error) { return "", nil }
func (*brokerStatusBackend) CreateVolume(context.Context, Object, *Object) error { return nil }
func (*brokerStatusBackend) InspectVolume(context.Context, Object) error          { return nil }
func (*brokerStatusBackend) Populate(context.Context, Object) error               { return nil }
func (b *brokerStatusBackend) RunGit(context.Context, AgentRequest) (Response, error) {
	b.gitCalls++
	return Response{}, nil
}
func (b *brokerStatusBackend) ConnectGit(context.Context, core.Environment, Object, string) error {
	b.connected++
	return nil
}

type brokerStatusEnvironmentStore struct{ environment core.Environment }

func (s *brokerStatusEnvironmentStore) GetEnvironment(context.Context, string) (core.Environment, error) {
	return s.environment, nil
}

func TestBrokerStatusIsReadOnlyAndRepairsMissingLocalSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &brokerStatusBackend{}
	repositories := NewRepositoryService(t.TempDir(), backend)
	repo := Object{Kind: "repo", ID: "repo", Repository: "repo", Remote: "https://github.com/example/repo.git", Branch: "main", NativeRef: "pool/source", Owner: strings.Repeat("a", 32), State: "ready"}
	if err := repositories.reserve(repo); err != nil {
		t.Fatal(err)
	}
	work := repo
	work.Kind = "work"
	work.ID = "work"
	work.NativeRef = "pool/work"
	work.Owner = strings.Repeat("b", 32)
	if err := repositories.reserve(work); err != nil {
		t.Fatal(err)
	}
	environment := core.Environment{Name: "dev", RuntimeRef: "instance:one", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:work"}}
	broker := NewBroker(repositories, &brokerStatusEnvironmentStore{environment: environment}, t.TempDir())
	if err := broker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer broker.Close()

	applicable, connected, err := broker.Status(ctx, "dev")
	if err != nil || !applicable || connected || backend.connected != 0 || backend.gitCalls != 0 {
		t.Fatalf("before connect: applicable=%v connected=%v connectCalls=%d gitCalls=%d err=%v", applicable, connected, backend.connected, backend.gitCalls, err)
	}
	if err := broker.Repair(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	applicable, connected, err = broker.Status(ctx, "dev")
	if err != nil || !applicable || !connected || backend.connected != 1 || backend.gitCalls != 0 {
		t.Fatalf("after connect: applicable=%v connected=%v connectCalls=%d gitCalls=%d err=%v", applicable, connected, backend.connected, backend.gitCalls, err)
	}

	if err := os.Remove(broker.socket("dev")); err != nil {
		t.Fatal(err)
	}
	applicable, connected, err = broker.Status(ctx, "dev")
	if err != nil || !applicable || connected || backend.gitCalls != 0 {
		t.Fatalf("missing socket: applicable=%v connected=%v gitCalls=%d err=%v", applicable, connected, backend.gitCalls, err)
	}
	if err := broker.Repair(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	applicable, connected, err = broker.Status(ctx, "dev")
	if err != nil || !applicable || !connected || backend.connected != 2 || backend.gitCalls != 0 {
		t.Fatalf("after repair: applicable=%v connected=%v connectCalls=%d gitCalls=%d err=%v", applicable, connected, backend.connected, backend.gitCalls, err)
	}
}

func TestBrokerRepairDoesNotDeleteNonSocketPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &brokerStatusBackend{}
	repositories := NewRepositoryService(t.TempDir(), backend)
	repo := Object{Kind: "repo", ID: "repo", Repository: "repo", Remote: "https://github.com/example/repo.git", Branch: "main", NativeRef: "pool/source", Owner: strings.Repeat("d", 32), State: "ready"}
	if err := repositories.reserve(repo); err != nil {
		t.Fatal(err)
	}
	work := repo
	work.Kind = "work"
	work.ID = "work"
	work.NativeRef = "pool/work"
	work.Owner = strings.Repeat("e", 32)
	if err := repositories.reserve(work); err != nil {
		t.Fatal(err)
	}
	environment := core.Environment{Name: "dev", RuntimeRef: "instance:one", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:work"}}
	broker := NewBroker(repositories, &brokerStatusEnvironmentStore{environment: environment}, t.TempDir())
	if err := broker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	if err := broker.Repair(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	path := broker.socket("dev")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := broker.Repair(ctx, "dev"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("repair error=%v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "sentinel" || backend.connected != 1 || backend.gitCalls != 0 {
		t.Fatalf("non-socket path changed: data=%q connectCalls=%d gitCalls=%d err=%v", data, backend.connected, backend.gitCalls, err)
	}
}

func TestBrokerStatusTreatsOfflineWorkspaceAsNotApplicable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &brokerStatusBackend{}
	repositories := NewRepositoryService(t.TempDir(), backend)
	work := Object{Kind: "work", ID: "offline", Repository: "repo", NativeRef: "pool/offline", Owner: strings.Repeat("c", 32), State: "ready"}
	if err := repositories.reserve(work); err != nil {
		t.Fatal(err)
	}
	environment := core.Environment{Name: "dev", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:offline"}}
	broker := NewBroker(repositories, &brokerStatusEnvironmentStore{environment: environment}, t.TempDir())
	if err := broker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer broker.Close()

	applicable, connected, err := broker.Status(ctx, "dev")
	if err != nil || applicable || connected || backend.connected != 0 || backend.gitCalls != 0 {
		t.Fatalf("offline status: applicable=%v connected=%v connectCalls=%d gitCalls=%d err=%v", applicable, connected, backend.connected, backend.gitCalls, err)
	}
}
