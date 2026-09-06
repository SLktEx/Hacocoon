package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type collectionBackend struct {
	localBackend
	service *RepositoryService
	t       *testing.T
	fail    bool
}

func (b *collectionBackend) CreateVolume(_ context.Context, object Object, _ *Object) error {
	data, err := os.ReadFile(b.service.path("work", "both"))
	if err != nil {
		b.t.Fatal(err)
	}
	var record Object
	if json.Unmarshal(data, &record) != nil || len(record.Members) != 2 {
		b.t.Fatal("missing durable collection")
	}
	found := false
	for _, member := range record.Members {
		if member.ID == object.ID && member.Owner == object.Owner && member.NativeRef == object.NativeRef && member.State == "creating" {
			found = true
		}
	}
	if !found {
		b.t.Fatal("member ownership not durable before creation")
	}
	if b.fail {
		return errors.New("ambiguous create")
	}
	return nil
}

func TestCollectionOwnershipAndMembersCannotBeLeasedSeparately(t *testing.T) {
	for _, fail := range []bool{false, true} {
		backend := &collectionBackend{t: t, fail: fail}
		service := NewRepositoryService(t.TempDir(), backend)
		backend.service = service
		for _, id := range []string{"one", "two"} {
			if err := service.save(Object{Kind: "repo", ID: id, Repository: id, Remote: "https://github.com/example/" + id + ".git", Branch: "main", NativeRef: "test-volume", Owner: strings.Repeat("a", 32), State: "ready"}); err != nil {
				t.Fatal(err)
			}
		}
		object, err := service.CopyWorkspaceSet(context.Background(), "both", []string{"one", "two"})
		if fail {
			if !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
			if _, err := service.Workspace(context.Background(), "both"); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
		} else if err != nil || object.State != "ready" {
			t.Fatalf("%+v %v", object, err)
		}
		for _, id := range []string{"both-one", "both-two"} {
			if _, err := service.Workspace(context.Background(), id); !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("member became separately leaseable: %v", err)
			}
		}
		if _, err := service.CopyWorkspaceSet(context.Background(), "duplicate", []string{"one", "one"}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
}

func TestTwoRepositoriesPushToTheirOwnRemotes(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git required")
	}
	root := t.TempDir()
	backend := localBackend{repos: filepath.Join(root, "repos"), workspaces: filepath.Join(root, "workspaces")}
	for _, dir := range []string{backend.repos, backend.workspaces, filepath.Join(root, "bin")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	service := NewRepositoryService(filepath.Join(root, "state"), backend)
	collection := Object{Kind: "work", ID: "both", Owner: strings.Repeat("b", 32), State: "ready"}
	for _, id := range []string{"one", "two"} {
		remote := filepath.Join(root, id+".git")
		source := filepath.Join(root, id)
		os.Mkdir(remote, 0700)
		os.Mkdir(source, 0700)
		testGit(t, remote, "init", "--bare", "--initial-branch=main")
		testGit(t, source, "init", "--initial-branch=main")
		testCommit(t, source, "initial.txt", id)
		testGit(t, source, "remote", "add", "origin", "file://"+remote)
		testGit(t, source, "push", "origin", "main")
		repo := Object{Kind: "repo", ID: id, Repository: id, Remote: "file://" + remote, Branch: "main", NativeRef: "test-volume", Owner: strings.Repeat("a", 32), State: "ready"}
		if err := service.save(repo); err != nil {
			t.Fatal(err)
		}
		if _, err := backend.RunGit(context.Background(), AgentRequest{Operation: "clone", Repository: id, Remote: repo.Remote, Branch: "main"}); err != nil {
			t.Fatal(err)
		}
		member := repo
		member.Kind = "work"
		member.ID = "both-" + id
		testGit(t, root, "clone", "--no-local", source, filepath.Join(backend.workspaces, member.ID))
		if _, err := backend.RunGit(context.Background(), AgentRequest{Operation: "workspace", Repository: id, Workspace: member.ID, Remote: repo.Remote, Branch: "main"}); err != nil {
			t.Fatal(err)
		}
		collection.Members = append(collection.Members, member)
	}
	if err := service.save(collection); err != nil {
		t.Fatal(err)
	}
	env := core.Environment{Name: "dev", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + collection.Owner), Path: "managed:both"}, RuntimeRef: "test:dev"}
	sockets, err := os.MkdirTemp("", "haco-multi-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockets)
	broker := NewBroker(service, singleEnvironment{env}, sockets)
	broker.Capabilities, err = capabilityapp.New(gitPolicy{}, nil, &gitAudit{}, broker)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := broker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	if err := broker.Connect(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	wrapper := "#!/bin/sh\nexec '" + strings.ReplaceAll(self, "'", "'\"'\"'") + "' -test.run=TestGitHelperProcess -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(root, "bin", "git-remote-haco"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(root, "bin")+":"+os.Getenv("PATH"))
	t.Setenv("HACO_GIT_TEST_HELPER", "1")
	t.Setenv("HACO_GIT_TEST_SOCKET", broker.socket("dev"))
	for _, id := range []string{"one", "two"} {
		work := filepath.Join(backend.workspaces, "both-"+id)
		testGit(t, work, "fetch", "origin")
		oid := testCommit(t, work, "change.txt", "only "+id)
		cmd := exec.CommandContext(ctx, "/usr/bin/git", "-C", work, "push", "origin", "main")
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		done := make(chan error, 1)
		go func() { done <- cmd.Run() }()
		for len(broker.Pending()) == 0 {
			select {
			case err := <-done:
				t.Fatalf("push ended before approval: %v %s", err, &output)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
		proposal := broker.Pending()[0]
		if proposal.Repository != id || proposal.NewOID != oid || proposal.Remote != "file://"+filepath.Join(root, id+".git") {
			t.Fatalf("wrong authority: %+v", proposal)
		}
		if err := broker.Decide(proposal.ID, true); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatalf("%v %s", err, &output)
		}
		if got := testGit(t, filepath.Join(root, id+".git"), "rev-parse", "main"); got != oid {
			t.Fatalf("got %s want %s", got, oid)
		}
	}
	if _, err := UnixExchange(broker.socket("dev"))(ctx, Request{Operation: "list", Repository: "foreign"}); err == nil {
		t.Fatal("foreign repository authorized")
	}
}
