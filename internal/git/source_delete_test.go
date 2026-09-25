package gitrepo

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

type sourceDeleteBackend struct {
	localBackend
	service    *RepositoryService
	busy, fail, forceFail bool
	deleted, forceDeleted bool
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
func (b *sourceDeleteBackend) ForceDeleteSourceVolume(_ context.Context, o Object) error {
	current, err := b.service.readObject("repo", o.ID)
	if err != nil || current.Owner != o.Owner || current.NativeRef != o.NativeRef {
		b.t.Fatal("forced delete target lost", current, err)
	}
	if b.forceFail {
		return core.ErrRuntimeUnavailable
	}
	b.forceDeleted = true
	return nil
}
func TestSourceDeletionAttemptsReferencedAndIncompleteCleanup(t *testing.T) {
	ctx := context.Background()
	b := &sourceDeleteBackend{t: t}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s

	o := Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/example/source.git", NativeRef: "pool/haco-repo-source", Owner: strings.Repeat("a", 32), State: "creating"}
	if err := s.reserve(o); err != nil {
		t.Fatal(err)
	}
	work := Object{Kind: "work", ID: "work", Repository: "source", Remote: o.Remote, Branch: "main", NativeRef: "pool/haco-work-work", Owner: strings.Repeat("b", 32), State: "creating"}
	if err := s.reserve(work); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListSources(ctx)
	if err != nil || len(all) != 1 || len(all[0].Workspaces) != 1 {
		t.Fatal(all, err)
	}

	// A confirmed delete intentionally ignores the blockers that the old
	// preflight used to report.
	b.busy = true
	if err := s.DeleteSource(ctx, o.ID, o.Owner); err != nil || !b.forceDeleted {
		t.Fatal("referenced/incomplete source was not deleted", err)
	}
	if _, err := s.readObject("repo", o.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("source receipt retained after successful deletion", err)
	}
	if _, err := s.readObject("work", work.ID); err != nil {
		t.Fatal("independent workspace record was removed", err)
	}

	failed := Object{Kind: "repo", ID: "failed", Repository: "failed", Remote: "https://github.com/example/failed.git", NativeRef: "pool/haco-repo-failed", Owner: strings.Repeat("c", 32), State: "creating"}
	if err := s.reserve(failed); err != nil {
		t.Fatal(err)
	}
	b.forceFail = true
	b.forceDeleted = false
	if err := s.DeleteSource(ctx, failed.ID, failed.Owner); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("native deletion failure was not retained", err)
	}
	if got, err := s.readObject("repo", failed.ID); err != nil || got.Owner != failed.Owner || got.State != "creating" {
		t.Fatal("failed forced cleanup lost its retry record", got, err)
	}

	// -f uses the current retained identity directly and does not require the
	// reviewed owner token from the preceding list operation.
	b.forceFail = false
	if err := s.ForceDeleteSource(ctx, failed.ID); err != nil || !b.forceDeleted {
		t.Fatal("force retry failed", err)
	}

	replacement := Object{Kind: "repo", ID: "replacement", Repository: "replacement", Remote: "https://github.com/example/replacement.git", NativeRef: "pool/haco-repo-replacement", Owner: strings.Repeat("d", 32), State: "ready"}
	if err := s.reserve(replacement); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSource(ctx, replacement.ID, strings.Repeat("e", 32)); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("reviewed delete accepted stale owner", err)
	}
	if err := s.ForceDeleteSource(ctx, replacement.ID); err != nil {
		t.Fatal("force delete did not use current source identity", err)
	}
	if err := s.ForceDeleteSource(ctx, replacement.ID); err != nil {
		t.Fatal("force delete was not idempotent after confirmed removal", err)
	}
}

func (b *sourceDeleteBackend) RunGit(context.Context, gitadapter.AgentRequest) (gitadapter.Response, error) {
	return gitadapter.Response{OID: "called"}, nil
}
func TestQueuedGitCannotUseSameNameSourceReplacement(t *testing.T) {
	b := &sourceDeleteBackend{t: t}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s
	original := Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/example/source.git", Branch: "main", NativeRef: "pool/haco-repo-source", Owner: strings.Repeat("a", 32), State: "ready"}
	if err := s.reserve(original); err != nil {
		t.Fatal(err)
	}
	req := gitadapter.AgentRequest{Operation: "list", Repository: original.ID, Remote: original.Remote, Branch: original.Branch}
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
