package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type importReconnectBackend struct {
	workspaceImportBackend
	connections int
}

func (b *importReconnectBackend) ConnectGit(context.Context, core.Environment, Object, string) error {
	b.connections++
	return nil
}

func TestImportedWorkspaceConnectsOnlyToExplicitMatchingSource(t *testing.T) {
	const remote = "https://github.com/SLktEx/Hacocoon-test.git"
	for _, mode := range []string{"matching", "remote-mismatch", "branch-mismatch", "offline"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			backend := &importReconnectBackend{workspaceImportBackend: workspaceImportBackend{ownershipBackend: ownershipBackend{t: t}}}
			repos := NewRepositoryService(t.TempDir(), backend)
			backend.service = repos
			savedRemote, savedBranch := remote, "main"
			if mode == "offline" {
				savedRemote, savedBranch = "", ""
			}
			work, err := repos.ImportWorkspace(ctx, "imported", "source", savedRemote, savedBranch, bytes.NewReader([]byte("saved data")))
			if err != nil {
				t.Fatal(err)
			}
			if backend.populated {
				t.Fatal("import ran source Git population")
			}
			environments := &identityEnvironmentStore{environment: core.Environment{Name: "dev", RuntimeRef: "instance:new", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:imported"}}}
			sockets, err := os.MkdirTemp("", "haco-reconnect-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(sockets)
			broker := NewBroker(repos, environments, sockets)
			if err := broker.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer broker.Close()
			if err := broker.Connect(ctx, "dev"); err == nil || backend.connections != 0 {
				t.Fatal("import acquired authority without a source", err)
			}
			if _, err := os.Stat(filepath.Join(repos.Root, "bindings", "dev.json")); !os.IsNotExist(err) {
				t.Fatal("missing route persisted a binding", err)
			}
			currentRemote, currentBranch := remote, "main"
			if mode == "remote-mismatch" {
				currentRemote = "https://github.com/example/different.git"
			}
			if mode == "branch-mismatch" {
				currentBranch = "other"
			}
			source, err := repos.Clone(ctx, "source", currentRemote, currentBranch)
			if err != nil {
				t.Fatal(err)
			}
			err = broker.Connect(ctx, "dev")
			if mode != "matching" {
				want := core.ErrCapabilityStale
				if mode == "offline" {
					want = core.ErrUnsupported
				}
				if !errors.Is(err, want) || backend.connections != 0 || len(broker.servers) != 0 {
					t.Fatal("same-name source adopted an unapproved route", err)
				}
				return
			}
			if err != nil || backend.connections != 1 {
				t.Fatal("matching current source could not connect", err)
			}
			bound := broker.servers["dev"].binding
			if !reflect.DeepEqual(bound.Repository, source) || !reflect.DeepEqual(bound.Workspace, work) {
				t.Fatal("connection lost current source owner or changed imported data identity")
			}
			if err := broker.validateBinding(ctx, bound); err != nil {
				t.Fatal(err)
			}
			environments.environment.RuntimeRef = "instance:replacement"
			if err := broker.validateBinding(ctx, bound); !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal("same-name Env reused imported connection", err)
			}
			environments.environment.RuntimeRef = "instance:new"
			source.Owner = strings.Repeat("f", 32)
			if err := repos.save(source); err != nil {
				t.Fatal(err)
			}
			if err := broker.validateBinding(ctx, bound); !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal("same-name source replacement reused connection", err)
			}
			after, err := repos.Get("work", work.ID)
			if err != nil || !reflect.DeepEqual(after, work) || backend.imports != 1 {
				t.Fatal("reconnection changed imported data registration", err)
			}
		})
	}
}
