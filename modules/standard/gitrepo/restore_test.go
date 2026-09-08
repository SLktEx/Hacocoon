package gitrepo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type savedBackend struct {
	localBackend
	t       *testing.T
	service *RepositoryService
	sources []SavedWorkspace
	fail    string
	deleted []string
	creates int
}

func (b *savedBackend) Plan(_ context.Context, kind, id string) (string, error) {
	return "pool/haco-" + kind + "-" + id, nil
}
func (b *savedBackend) SavedWorkspaces(context.Context, core.Snapshot) ([]SavedWorkspace, error) {
	return b.sources, nil
}
func (b *savedBackend) durable(member Object, state string) {
	b.t.Helper()
	raw, err := os.ReadFile(b.service.path("work", "restored"))
	if err != nil {
		b.t.Fatal(err)
	}
	var object Object
	if json.Unmarshal(raw, &object) != nil {
		b.t.Fatal("invalid receipt")
	}
	for _, m := range object.Copies() {
		if m.ID == member.ID && m.Owner == member.Owner && m.NativeRef == member.NativeRef && m.State == state {
			return
		}
	}
	b.t.Fatalf("missing durable %s receipt for %s: %s", state, member.ID, raw)
}
func (b *savedBackend) CreateSavedWorkspace(_ context.Context, member Object, _ SavedWorkspace) error {
	b.durable(member, "creating")
	b.creates++
	if b.fail == "copy" || b.fail == "cleanup" {
		return core.ErrRecoveryRequired
	}
	return nil
}
func (b *savedBackend) InspectVolume(_ context.Context, member Object) error {
	b.durable(member, "created")
	if b.fail == "verify" {
		return core.ErrCapabilityStale
	}
	return nil
}
func (b *savedBackend) Populate(context.Context, Object) error {
	b.t.Fatal("restore rewrote Git data")
	return nil
}
func (b *savedBackend) DeleteRestoredWorkspaceVolume(_ context.Context, member Object) error {
	b.deleted = append(b.deleted, member.ID)
	if b.fail == "cleanup" && member.ID == "restored-one" {
		return core.ErrCapabilityStale
	}
	return nil
}
func TestRestoredWorkspaceOwnershipAndCleanup(t *testing.T) {
	for _, mode := range []string{"single", "set", "copy", "verify", "cleanup"} {
		t.Run(mode, func(t *testing.T) {
			backend := &savedBackend{t: t, fail: mode}
			for _, name := range []string{"one", "two"} {
				backend.sources = append(backend.sources, SavedWorkspace{Repository: name, Remote: "https://github.com/example/" + name + ".git", Branch: "main", Component: core.SnapshotComponent{Role: "workspace:" + name, State: "verified"}})
			}
			if mode == "single" {
				backend.sources = backend.sources[:1]
			}
			service := NewRepositoryService(t.TempDir(), backend)
			backend.service = service
			saved := core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready"}
			object, err := service.RestoreWorkspace(context.Background(), "restored", saved)
			if mode == "single" || mode == "set" {
				if err != nil || object.State != "ready" || object.RestoredFrom != saved.ID {
					t.Fatalf("%+v %v", object, err)
				}
				reopened := NewRepositoryService(service.Root, backend)
				got, err := reopened.Get("work", "restored")
				if err != nil || got.Owner != object.Owner {
					t.Fatal(got, err)
				}
				if err := service.CleanupRestoredWorkspace(context.Background(), "restored"); !errors.Is(err, core.ErrIncompatibleState) {
					t.Fatal("published copy accepted for failure cleanup", err)
				}
				if _, err := service.RestoreWorkspace(context.Background(), "restored", saved); !errors.Is(err, core.ErrAlreadyExists) {
					t.Fatal("overwrote existing", err)
				}
			} else {
				if err == nil {
					t.Fatal("failure accepted")
				}
				if len(backend.deleted) != 2 {
					t.Fatal("independent cleanup skipped", backend.deleted)
				}
				if mode == "cleanup" {
					if !errors.Is(err, core.ErrRecoveryRequired) {
						t.Fatal(err)
					}
					if _, err := service.Get("work", "restored"); !errors.Is(err, core.ErrRecoveryRequired) {
						t.Fatal("ownership discarded", err)
					}
					backend.fail = ""
					if err := service.CleanupRestoredWorkspace(context.Background(), "restored"); err != nil {
						t.Fatal(err)
					}
				} else if errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("cleaned failure retains recovery requirement", err)
				}
				if _, err := service.Get("work", "restored"); !errors.Is(err, core.ErrNotFound) {
					t.Fatal("cleanup retained record", err)
				}
			}
		})
	}
}
func TestRestoredWorkspaceRejectsInvalidSavedMetadata(t *testing.T) {
	backend := &savedBackend{t: t, sources: []SavedWorkspace{{Repository: "one", Remote: "https://token@github.com/example/one.git", Branch: "main", Component: core.SnapshotComponent{State: "verified"}}}}
	service := NewRepositoryService(t.TempDir(), backend)
	backend.service = service
	_, err := service.RestoreWorkspace(context.Background(), "restored", core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready"})
	if err == nil || backend.creates != 0 {
		t.Fatal("unsafe metadata reached provider", err)
	}
}

func (b *savedBackend) PlanSavedWorkspace(_ context.Context, id string, _ SavedWorkspace) (string, error) {
	return "pool/haco-work-" + id, nil
}
