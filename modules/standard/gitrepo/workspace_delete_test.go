package gitrepo

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

type deleteWorkBackend struct {
	localBackend
	service *RepositoryService
	t       *testing.T
	calls   []string
	fail    bool
}

func (b *deleteWorkBackend) DeleteWorkspaceVolume(_ context.Context, o Object) error {
	current, err := b.service.readObject("work", "both")
	if err != nil || current.State != "deleting" {
		b.t.Fatal("missing durable delete receipt", current, err)
	}
	b.calls = append(b.calls, o.ID)
	if o.Repository == "two" && b.fail {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func TestWorkspaceDeleteKeepsPartialOwnershipAndRefusesReplacement(t *testing.T) {
	ctx := context.Background()
	backend := &deleteWorkBackend{t: t, fail: true}
	s := NewRepositoryService(t.TempDir(), backend)
	backend.service = s
	owner := strings.Repeat("a", 32)
	object := Object{Kind: "work", ID: "both", Owner: owner, State: "ready"}
	for _, name := range []string{"one", "two"} {
		object.Members = append(object.Members, Object{Kind: "work", ID: "both-" + name, Repository: name, Remote: "https://github.com/example/" + name + ".git", Branch: "main", NativeRef: "pool/haco-work-both-" + name, Owner: strings.Repeat("b", 32), State: "ready"})
	}
	if err := s.reserve(object); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(ctx, "both", strings.Repeat("c", 32)); !errors.Is(err, core.ErrCapabilityStale) || len(backend.calls) != 0 {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(ctx, "both", owner); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	got, err := s.Get("work", "both")
	if !errors.Is(err, core.ErrRecoveryRequired) || got.Owner != owner || got.State != "deleting" {
		t.Fatal(got, err)
	}
	all, err := s.ListWorkspaces(ctx)
	if err != nil || len(all) != 1 || all[0].State != "deleting" {
		t.Fatal(all, err)
	}
	backend.fail = false
	if err := s.DeleteWorkspace(ctx, "both", owner); err != nil {
		t.Fatal(err)
	}
	all, err = s.ListWorkspaces(ctx)
	if err != nil || len(all) != 0 {
		t.Fatal(all, err)
	}
	if err := s.reserve(object); err != nil {
		t.Fatal(err)
	}
	object.Owner = strings.Repeat("d", 32)
	if err := s.save(object); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(ctx, "both", owner); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("recycled name accepted", err)
	}
}
func TestWorkspaceDeleteRefusesActivePreparation(t *testing.T) {
	backend := &deleteWorkBackend{t: t}
	s := NewRepositoryService(t.TempDir(), backend)
	backend.service = s
	o := Object{Kind: "work", ID: "both", Repository: "repo", Remote: "https://github.com/example/repo.git", Branch: "main", NativeRef: "pool/haco-work-both", Owner: strings.Repeat("a", 32), State: "creating"}
	if err := s.reserve(o); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(context.Background(), o.ID, o.Owner); !errors.Is(err, core.ErrRecoveryRequired) || len(backend.calls) != 0 {
		t.Fatal(err)
	}
}
