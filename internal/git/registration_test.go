package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/policy"
)

type gitVolumeBackend struct{ localBackend }

func (b gitVolumeBackend) CreateVolume(ctx context.Context, target Object, source *Object) error {
	if source == nil {
		return nil
	}
	destination := filepath.Join(b.workspaces, target.ID)
	if err := os.Mkdir(destination, 0700); err != nil {
		return err
	}
	return exec.CommandContext(ctx, "/bin/cp", "-a", "--", filepath.Join(b.repos, source.ID)+"/.", destination).Run()
}

func (b gitVolumeBackend) Populate(ctx context.Context, object Object) error {
	op := "clone"
	if object.Kind == "work" {
		op = "workspace"
	}
	_, err := b.RunGit(ctx, gitadapter.AgentRequest{Operation: op, Repository: object.Repository, Workspace: object.ID, Remote: object.Remote, Branch: object.Branch})
	return err
}

func TestOneRegisteredRepositorySupportsIndependentWorkspaceBranches(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git is required")
	}
	root := t.TempDir()
	remote, seed := filepath.Join(root, "remote.git"), filepath.Join(root, "seed")
	b := gitVolumeBackend{localBackend{repos: filepath.Join(root, "repos"), workspaces: filepath.Join(root, "workspaces")}}
	for _, dir := range []string{remote, seed, b.repos, b.workspaces} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=trunk")
	testGit(t, seed, "init", "--initial-branch=trunk")
	trunk := testCommit(t, seed, "data", "trunk\n")
	testGit(t, seed, "remote", "add", "origin", "file://"+remote)
	testGit(t, seed, "push", "origin", "trunk")
	testGit(t, seed, "checkout", "-b", "feature/one")
	feature := testCommit(t, seed, "data", "feature\n")
	testGit(t, seed, "push", "origin", "feature/one")
	service := NewRepositoryService(filepath.Join(root, "state"), b)
	var progress bytes.Buffer
	ctx := gitadapter.WithProgress(context.Background(), &progress)
	repo, err := service.Add(ctx, "sample", "file://"+remote)
	if err != nil || repo.Branch != "" || repo.State != "ready" {
		t.Fatal(repo, err)
	}
	if !strings.Contains(progress.String(), "Cloning into managed repository") || !strings.Contains(progress.String(), "Receiving objects:") {
		t.Fatal("no native transfer progress:", &progress)
	}
	data, err := os.ReadFile(service.path("repo", "sample"))
	if err != nil || bytes.Contains(data, []byte(`"branch"`)) {
		t.Fatal("repository persisted branch identity", string(data), err)
	}
	if got := testGit(t, filepath.Join(b.repos, "sample"), "rev-parse", "refs/remotes/origin/feature/one"); got != feature {
		t.Fatal("registration lost other branches", got)
	}
	// A branch created after registration must also resolve from this source.
	testGit(t, seed, "checkout", "-b", "later")
	later := testCommit(t, seed, "data", "later\n")
	testGit(t, seed, "push", "origin", "later")
	for _, tc := range []struct{ name, branch, wantBranch, oid string }{
		{"default-work", "", "trunk", trunk}, {"feature-work", "feature/one", "feature/one", feature}, {"later-work", "later", "later", later},
	} {
		work, err := service.CopyWorkspaceBranch(ctx, tc.name, "sample", tc.branch)
		if err != nil || work.Branch != tc.wantBranch {
			t.Fatal(work, err)
		}
		dir := filepath.Join(b.workspaces, tc.name)
		if got := testGit(t, dir, "rev-parse", "HEAD"); got != tc.oid {
			t.Fatal("wrong checkout", got, tc.oid)
		}
		if got := testGit(t, dir, "remote", "get-url", "origin"); got != "haco://sample" {
			t.Fatal("trusted URL escaped into Workspace", got)
		}
		env := core.Environment{Name: "dev", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:" + work.ID}}
		broker := NewBroker(service, singleEnvironment{env}, "")
		capabilities, err := capabilityapp.New(gitPolicy{}, nil, &gitAudit{}, broker)
		if err != nil {
			t.Fatal(err)
		}
		broker.Capabilities = capabilities
		bound := binding{Environment: env, Workspace: work, Repository: repo}
		if err := broker.validateBinding(ctx, bound); err != nil {
			t.Fatal(err)
		}
		listed, err := broker.exchange(ctx, bound, gitadapter.Request{Operation: "list", Repository: repo.ID})
		if err != nil || listed.Ref != "refs/heads/"+tc.wantBranch || listed.OID != tc.oid || len(listed.Heads) != 3 {
			t.Fatal("broker used source branch instead of Workspace route", listed, err)
		}
		wrong := "refs/heads/not-selected"
		if _, err := broker.exchange(ctx, bound, gitadapter.Request{Operation: "fetch", Repository: repo.ID, Ref: wrong, NewOID: trunk}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal("malformed fetch bypassed head validation", err)
		}
	}
	if _, err := service.CopyWorkspaceBranch(ctx, "bad", "sample", "--upload-pack=evil"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}
