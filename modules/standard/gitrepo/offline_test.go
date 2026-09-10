package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestOfflineWorkspaceImportAndSavedRestore(t *testing.T) {
	b := &workspaceImportBackend{ownershipBackend: ownershipBackend{t: t}}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s
	o, err := s.ImportWorkspace(context.Background(), "imported", "repo", "", "", bytes.NewReader([]byte("saved data")))
	if err != nil || o.State != "ready" || o.Remote != "" || o.Branch != "" || b.populated {
		t.Fatal(o, err)
	}
	if _, err := s.Get("work", o.ID); err != nil {
		t.Fatal(err)
	}
	repo := o
	repo.Kind = "repo"
	if validObject(repo) {
		t.Fatal("empty source repository accepted")
	}
	for _, pair := range [][2]string{{"", "main"}, {"https://github.com/example/repo.git", ""}} {
		if ValidWorkspaceRouting(pair[0], pair[1]) {
			t.Fatal("partial routing accepted")
		}
	}
	savedBackend := &savedBackend{t: t, sources: []SavedWorkspace{{Repository: "repo", Component: core.SnapshotComponent{Role: "workspace:repo", State: "verified"}}}}
	restores := NewRepositoryService(t.TempDir(), savedBackend)
	savedBackend.service = restores
	restores.SnapshotCatalog = &workspaceCopyCatalog{}
	restored, err := restores.RestoreWorkspace(context.Background(), "restored", core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready"})
	if err != nil || restored.State != "ready" || restored.Remote != "" || restored.Branch != "" {
		t.Fatal(restored, err)
	}
}

type offlineBrokerBackend struct {
	localBackend
	connected bool
}

func (b *offlineBrokerBackend) ConnectGit(context.Context, core.Environment, Object, string) error {
	b.connected = true
	return nil
}
func TestOfflineBrokerExcludesSameNameSourceAndChecksOnlineRouting(t *testing.T) {
	for _, mode := range []string{"offline", "mixed", "remote-drift", "branch-drift"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			backend := &offlineBrokerBackend{}
			s := NewRepositoryService(t.TempDir(), backend)
			makeRepo := func(id string) Object {
				return Object{Kind: "repo", ID: id, Repository: id, Remote: "https://github.com/example/" + id + ".git", Branch: "main", Owner: strings.Repeat("a", 32), NativeRef: "pool/" + id, State: "ready"}
			}
			offlineRepo := makeRepo("offline")
			onlineRepo := makeRepo("online")
			for _, r := range []Object{offlineRepo, onlineRepo} {
				if err := s.reserve(r); err != nil {
					t.Fatal(err)
				}
			}
			offline := offlineRepo
			offline.Kind = "work"
			offline.ID = "both-offline"
			offline.Owner = strings.Repeat("b", 32)
			offline.NativeRef = "pool/offline-copy"
			offline.Remote = ""
			offline.Branch = ""
			online := onlineRepo
			online.Kind = "work"
			online.ID = "both-online"
			online.Owner = strings.Repeat("c", 32)
			online.NativeRef = "pool/online-copy"
			work := Object{Kind: "work", ID: "both", Owner: strings.Repeat("d", 32), State: "ready", Members: []Object{offline, online}}
			if mode == "offline" {
				work = offline
				work.ID = "both"
			}
			if mode == "remote-drift" {
				work.Members[1].Remote = "https://github.com/example/other.git"
			}
			if mode == "branch-drift" {
				work.Members[1].Branch = "other"
			}
			if err := s.reserve(work); err != nil {
				t.Fatal(err)
			}
			envs := &identityEnvironmentStore{environment: core.Environment{Name: "dev", RuntimeRef: "instance:one", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:both"}}}
			sockets, err := os.MkdirTemp("", "haco-offline-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(sockets)
			b := NewBroker(s, envs, sockets)
			if err := b.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			err = b.Connect(ctx, "dev")
			if mode != "mixed" {
				want := core.ErrCapabilityStale
				if mode == "offline" {
					want = core.ErrUnsupported
				}
				if !errors.Is(err, want) || backend.connected || len(b.servers) != 0 {
					t.Fatal("offline/drift created authority", err)
				}
				if _, err := os.Stat(filepath.Join(s.Root, "bindings", "dev.json")); !os.IsNotExist(err) {
					t.Fatal("refused route persisted binding", err)
				}
				return
			}
			if err != nil || !backend.connected {
				t.Fatal(err)
			}
			bound := b.servers["dev"].binding
			if len(bound.Repositories) != 1 || bound.Repositories[0].ID != "online" {
				t.Fatal("offline source adopted", bound.Repositories)
			}
			if _, err := b.exchange(ctx, bound, Request{Operation: "list", Repository: "offline"}); !errors.Is(err, core.ErrPolicyDenied) {
				t.Fatal("offline request reached capability", err)
			}
			if err := b.validateBinding(ctx, bound); err != nil {
				t.Fatal(err)
			}
			onlineRepo.Remote = "https://github.com/example/replaced.git"
			if err := s.save(onlineRepo); err != nil {
				t.Fatal(err)
			}
			if err := b.validateBinding(ctx, bound); !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal("changed source reused", err)
			}
		})
	}
}
func TestOfflineWorkspaceDoesNotRetainUnrelatedHostSource(t *testing.T) {
	b := &sourceDeleteBackend{t: t}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s
	source := Object{Kind: "repo", ID: "repo", Repository: "repo", Remote: "https://github.com/example/repo.git", Branch: "main", NativeRef: "pool/source", Owner: strings.Repeat("a", 32), State: "ready"}
	if err := s.reserve(source); err != nil {
		t.Fatal(err)
	}
	work := source
	work.Kind = "work"
	work.ID = "work"
	work.NativeRef = "pool/data"
	work.Owner = strings.Repeat("b", 32)
	work.Remote = ""
	work.Branch = ""
	if err := s.reserve(work); err != nil {
		t.Fatal(err)
	}
	uses, err := s.ListSources(context.Background())
	if err != nil || len(uses) != 1 || len(uses[0].Workspaces) != 0 {
		t.Fatal(uses, err)
	}
	if err := s.DeleteSource(context.Background(), source.ID, source.Owner); err != nil || !b.deleted {
		t.Fatal(err)
	}
	got, err := s.Get("work", "work")
	if err != nil || got.Owner != work.Owner {
		t.Fatal("offline data removed", err)
	}
}
