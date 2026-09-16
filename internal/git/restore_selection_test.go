package gitrepo

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type selectionBackend struct {
	savedBackend
	added, populated []string
	addFailure       bool
}

func (b *selectionBackend) InspectVolume(ctx context.Context, object Object) error {
	if object.Kind == "repo" {
		return nil
	}
	return b.savedBackend.InspectVolume(ctx, object)
}
func (b *selectionBackend) CreateVolume(_ context.Context, member Object, source *Object) error {
	b.durable(member, "creating")
	if source == nil || source.ID != "three" || source.Owner != strings.Repeat("c", 32) || b.service.SnapshotCatalog.(*workspaceCopyCatalog).owner == "" {
		b.t.Fatal("addition without exact registered source and saved-data reservation", source)
	}
	b.added = append(b.added, source.ID)
	if b.addFailure {
		return core.ErrRecoveryRequired
	}
	return nil
}
func (b *selectionBackend) Populate(_ context.Context, member Object) error {
	b.durable(member, "created")
	if member.Repository != "three" {
		b.t.Fatal("rewrote the saved member's Git state")
	}
	b.populated = append(b.populated, member.Repository)
	return nil
}
func (b *selectionBackend) CheckSourceDeletion(context.Context, Object) error {
	b.t.Fatal("referenced source reached provider deletion")
	return nil
}
func (b *selectionBackend) DeleteSourceVolume(context.Context, Object) error {
	b.t.Fatal("referenced source was deleted")
	return nil
}

func TestRestoreSelectedMembershipKeepsSavedDataAndPinsAdditions(t *testing.T) {
	for _, mode := range []string{"replace", "remove", "all", "unknown-copy"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			b := &selectionBackend{savedBackend: savedBackend{t: t}}
			for _, name := range []string{"one", "two"} {
				b.sources = append(b.sources, SavedWorkspace{Repository: name, Remote: "https://github.com/example/" + name + ".git", Branch: "main", Component: core.SnapshotComponent{State: "verified"}})
			}
			s := NewRepositoryService(t.TempDir(), b)
			b.service = s
			s.SnapshotCatalog = &workspaceCopyCatalog{}
			repo := Object{Kind: "repo", ID: "three", Repository: "three", Remote: "https://github.com/example/three.git", Branch: "main", Owner: strings.Repeat("c", 32), NativeRef: "pool/haco-repo-three", State: "ready"}
			if err := s.reserve(repo); err != nil {
				t.Fatal(err)
			}
			selection := []string{"one", "three"}
			switch mode {
			case "remove":
				selection = []string{"two"}
			case "all":
				selection = nil
			case "unknown-copy":
				selection = []string{"three", "one"}
				b.addFailure = true
				b.fail = "cleanup"
			}
			prepared := false
			saved := core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready"}
			object, err := s.RestoreWorkspaceSelectionWithData(ctx, "restored", saved, selection, func(_ context.Context, work core.Workspace) error {
				if _, err := s.Get("work", "restored"); !errors.Is(err, core.ErrRecoveryRequired) || work.ID == "" {
					t.Fatal("published before associated data", err)
				}
				prepared = true
				return nil
			})
			if mode == "unknown-copy" {
				if !errors.Is(err, core.ErrRecoveryRequired) || prepared || len(b.added) != 1 {
					t.Fatal(object, err)
				}
				// A fresh registry instance models restart. Its durable destination
				// membership must still pin the newly selected source.
				restarted := NewRepositoryService(s.Root, b)
				if err := restarted.DeleteSource(ctx, repo.ID, repo.Owner); !errors.Is(err, core.ErrStorageBusy) {
					t.Fatal("lost source pin", err)
				}
				if s.SnapshotCatalog.(*workspaceCopyCatalog).owner == "" {
					t.Fatal("lost snapshot reservation")
				}
				return
			}
			if err != nil || !prepared || object.State != "ready" {
				t.Fatal(object, err)
			}
			var actual []string
			for _, member := range object.Copies() {
				actual = append(actual, member.Repository)
			}
			expected := selection
			if expected == nil {
				expected = []string{"one", "two"}
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatal(actual, expected)
			}
			if mode == "replace" && (!reflect.DeepEqual(b.added, []string{"three"}) || !reflect.DeepEqual(b.populated, b.added) || b.creates != 1) {
				t.Fatal("wrong copy source", b.added, b.populated, b.creates)
			}
			if mode != "replace" && (len(b.added) != 0 || len(b.populated) != 0) {
				t.Fatal("unexpected addition")
			}
		})
	}
}

func TestRestoreSelectionRejectsInvalidOrMissingSourcesBeforeReservation(t *testing.T) {
	for _, selection := range [][]string{{}, {"one", "one"}, {"../one"}, {"missing"}, {"a", "b", "c", "d", "e", "f", "g", "h", "i"}} {
		b := &selectionBackend{savedBackend: savedBackend{t: t, sources: []SavedWorkspace{{Repository: "one", Component: core.SnapshotComponent{State: "verified"}}}}}
		s := NewRepositoryService(t.TempDir(), b)
		b.service = s
		s.SnapshotCatalog = &workspaceCopyCatalog{}
		_, err := s.RestoreWorkspaceSelectionWithData(context.Background(), "restored", core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready"}, selection, nil)
		if err == nil || b.creates != 0 || len(b.added) != 0 {
			t.Fatal("invalid selection reached provider", selection, err)
		}
		if _, err := s.Get("work", "restored"); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("invalid selection reserved target", err)
		}
	}
}
