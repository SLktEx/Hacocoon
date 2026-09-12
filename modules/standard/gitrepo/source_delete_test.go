package gitrepo

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"strings"
	"testing"
)

type sourceDeleteBackend struct {
	localBackend
	service    *RepositoryService
	busy, fail bool
	deleted    bool
	t          *testing.T
}

func (b *sourceDeleteBackend) CheckSourceDeletion(context.Context, Object) error {
	if b.busy {
		return core.ErrStorageBusy
	}
	return nil
}
func (b *sourceDeleteBackend) DeleteSourceVolume(_ context.Context, o Object) error {
	current, err := b.service.readObject("repo", o.ID)
	if err != nil || current.Owner != o.Owner || current.State != "deleting" {
		b.t.Fatal("owned delete receipt missing", current, err)
	}
	if b.fail {
		return core.ErrRuntimeUnavailable
	}
	b.deleted = true
	return nil
}
func TestSourceDeletionPreservesReferencesAndAmbiguousCleanup(t *testing.T) {
	ctx := context.Background()
	b := &sourceDeleteBackend{t: t}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s
	o := Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/example/source.git", Branch: "main", NativeRef: "pool/haco-repo-source", Owner: strings.Repeat("a", 32), State: "ready"}
	if err := s.reserve(o); err != nil {
		t.Fatal(err)
	}
	work := o
	work.Kind = "work"
	work.ID = "work"
	work.NativeRef = "pool/haco-work-work"
	work.State = "creating"
	if err := s.reserve(work); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListSources(ctx)
	if err != nil || len(all) != 1 || len(all[0].Workspaces) != 1 {
		t.Fatal(all, err)
	}
	if err := s.DeleteSource(ctx, o.ID, o.Owner); !errors.Is(err, core.ErrStorageBusy) || b.deleted {
		t.Fatal(err)
	}
	if err := os.Remove(s.path("work", work.ID)); err != nil {
		t.Fatal(err)
	}
	b.busy = true
	if err := s.DeleteSource(ctx, o.ID, o.Owner); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal(err)
	}
	if got, err := s.Get("repo", o.ID); err != nil || got.State != "ready" {
		t.Fatal(got, err)
	}
	b.busy = false
	b.fail = true
	if err := s.DeleteSource(ctx, o.ID, o.Owner); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	got, err := s.readObject("repo", o.ID)
	if err != nil || got.Owner != o.Owner || got.State != "deleting" {
		t.Fatal(got, err)
	}
	if err := s.DeleteSource(ctx, o.ID, strings.Repeat("b", 32)); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	b.fail = false
	if err := s.DeleteSource(ctx, o.ID, o.Owner); err != nil || !b.deleted {
		t.Fatal(err)
	}
	o.Owner = strings.Repeat("c", 32)
	if err := s.reserve(o); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSource(ctx, o.ID, strings.Repeat("a", 32)); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
}

func (b *sourceDeleteBackend) RunGit(context.Context, AgentRequest) (Response, error) {
	return Response{OID: "called"}, nil
}
func TestQueuedGitCannotUseSameNameSourceReplacement(t *testing.T) {
	b := &sourceDeleteBackend{t: t}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s
	original := Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/example/source.git", Branch: "main", NativeRef: "pool/haco-repo-source", Owner: strings.Repeat("a", 32), State: "ready"}
	if err := s.reserve(original); err != nil {
		t.Fatal(err)
	}
	req := AgentRequest{Operation: "list", Repository: original.ID, Remote: original.Remote, Branch: original.Branch}
	if got, err := s.RunGit(context.Background(), original, req); err != nil || got.OID != "called" {
		t.Fatal(got, err)
	}
	replacement := original
	replacement.Owner = strings.Repeat("b", 32)
	if err := s.save(replacement); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunGit(context.Background(), original, req); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("old validated request reached new source", err)
	}
	req.Remote = "https://github.com/example/other.git"
	if _, err := s.RunGit(context.Background(), replacement, req); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("source identity did not bind upstream", err)
	}
}
