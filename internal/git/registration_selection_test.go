package gitrepo

import (
	"context"
	"errors"
	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"strings"
	"testing"
)

type registrationBackend struct {
	localBackend
	updates int
}

func (b *registrationBackend) InspectVolume(context.Context, Object) error { return nil }
func (b *registrationBackend) RunGit(context.Context, gitadapter.AgentRequest) (gitadapter.Response, error) {
	b.updates++
	return gitadapter.Response{Ref: "refs/heads/main", OID: strings.Repeat("a", 40)}, nil
}
func TestRegistrationSelectionDoesNotChangeExistingGitAuthority(t *testing.T) {
	ctx := context.Background()
	b := &registrationBackend{}
	s := NewRepositoryService(t.TempDir(), b)
	source := Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/Foo/Bar.git", NativeRef: "pool/source", Owner: strings.Repeat("a", 32), State: "ready"}
	if err := s.reserve(source); err != nil {
		t.Fatal(err)
	}
	work := Object{Kind: "work", ID: "work", Repository: "source", Remote: source.Remote, Branch: "main", NativeRef: "pool/work", Owner: strings.Repeat("b", 32), State: "ready"}
	if err := s.reserve(work); err != nil {
		t.Fatal(err)
	}
	if err := s.UnregisterSource(ctx, source.ID, source.Owner); err != nil {
		t.Fatal(err)
	}
	if all, err := s.ListSources(ctx); err != nil || len(all) != 0 {
		t.Fatal(all, err)
	}
	if got, err := s.Get("repo", "source"); err != nil || !reflect.DeepEqual(got, source) {
		t.Fatal("existing authority changed", got, err)
	}
	for _, remote := range []string{"git@github.com:foo/bar.git", "ssh://git@github.com/foo/bar.git"} {
		got, err := s.Add(ctx, "", remote)
		if err != nil || !reflect.DeepEqual(got, source) {
			t.Fatal(got, err)
		}
	}
	if b.updates != 2 {
		t.Fatal("duplicate add skipped fetch", b.updates)
	}
	if got, err := s.Get("work", "work"); err != nil || !reflect.DeepEqual(got, work) {
		t.Fatal("registration changed Workspace", got, err)
	}
}

type retryRegistrationBackend struct {
	registrationBackend
	attempts int
	failure  error
}

func (b *retryRegistrationBackend) PrepareRepository(_ context.Context, o Object) error {
	b.attempts++
	return b.failure
}
func TestRegistrationRetryKeepsOwnedIdentityAcrossFailure(t *testing.T) {
	ctx := context.Background()
	b := &retryRegistrationBackend{failure: errors.New("network unavailable")}
	s := NewRepositoryService(t.TempDir(), b)
	source := Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/example/project", NativeRef: "pool/source", Owner: strings.Repeat("a", 32), State: "created"}
	if err := s.reserve(source); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, "", source.Remote); err == nil {
		t.Fatal("network failure accepted")
	}
	if got, err := s.Get("repo", source.ID); !errors.Is(err, core.ErrRecoveryRequired) || got.State != "created" || got.Owner != source.Owner {
		t.Fatal(got, err)
	}
	b.failure = nil
	got, err := s.Add(ctx, "", source.Remote)
	if err != nil || got.State != "ready" || got.Owner != source.Owner || got.NativeRef != source.NativeRef || b.attempts != 2 {
		t.Fatal(got, err, b.attempts)
	}
	if _, err = s.Get("repo", "absent"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}
