package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type localBackend struct{ repos, workspaces string }

func (localBackend) Plan(context.Context, string, string) (string, error)               { return "test-volume", nil }
func (localBackend) CreateVolume(context.Context, Object, *Object) error                { return nil }
func (localBackend) InspectVolume(context.Context, Object) error                        { return nil }
func (localBackend) Populate(context.Context, Object) error                             { return nil }
func (localBackend) ConnectGit(context.Context, core.Environment, Object, string) error { return nil }
func (b localBackend) RunGit(ctx context.Context, request AgentRequest) (Response, error) {
	return RunAgent(ctx, request, b.repos, b.workspaces)
}

type singleEnvironment struct{ environment core.Environment }

func (s singleEnvironment) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	if name != s.environment.Name {
		return core.Environment{}, core.ErrNotFound
	}
	return s.environment, nil
}

type gitPolicy struct{}

func (gitPolicy) Evaluate(_ context.Context, req core.CapabilityRequest) (core.PolicyEvaluation, error) {
	decision := core.PolicyAllow
	if req.Action == "push" {
		decision = core.PolicyRequireApproval
	}
	return core.PolicyEvaluation{Decision: decision}, nil
}

type gitAudit struct {
	mu     sync.Mutex
	events []core.CapabilityAuditEvent
}

func (a *gitAudit) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
	return nil
}

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("/usr/bin/git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=PoC", "GIT_AUTHOR_EMAIL=poc@example.invalid", "GIT_COMMITTER_NAME=PoC", "GIT_COMMITTER_EMAIL=poc@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func testCommit(t *testing.T, dir, file, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", "--", file)
	testGit(t, dir, "commit", "-m", "update "+file)
	return testGit(t, dir, "rev-parse", "HEAD")
}

func TestGitHelperProcess(t *testing.T) {
	if os.Getenv("HACO_GIT_TEST_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	err := Helper(context.Background(), args[1:], os.Stdin, os.Stdout, os.Stderr, UnixExchange(os.Getenv("HACO_GIT_TEST_SOCKET")))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestOrdinaryGitFetchPullDeniedAndPinnedPush(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git is required")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	for _, dir := range []string{remote, seed, filepath.Join(root, "repos"), filepath.Join(root, "workspaces")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=main")
	testGit(t, seed, "init", "--initial-branch=main")
	initial := testCommit(t, seed, "hello.txt", "initial\n")
	testGit(t, seed, "remote", "add", "origin", "file://"+remote)
	testGit(t, seed, "push", "origin", "main")
	backend := localBackend{repos: filepath.Join(root, "repos"), workspaces: filepath.Join(root, "workspaces")}
	repo := Object{Kind: "repo", ID: "demo", Repository: "demo", Remote: "file://" + remote, Branch: "main", NativeRef: "test-volume", Owner: strings.Repeat("a", 32), State: "ready"}
	work := repo
	work.Kind = "work"
	work.ID = "work"
	work.Owner = strings.Repeat("b", 32)
	if _, err := backend.RunGit(context.Background(), AgentRequest{Operation: "clone", Repository: repo.ID, Remote: repo.Remote, Branch: repo.Branch}); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(backend.workspaces, "work")
	testGit(t, root, "clone", "--no-local", filepath.Join(backend.repos, "demo"), workspace)
	if _, err := backend.RunGit(context.Background(), AgentRequest{Operation: "workspace", Repository: repo.ID, Workspace: work.ID, Remote: repo.Remote, Branch: repo.Branch}); err != nil {
		t.Fatal(err)
	}
	repositories := NewRepositoryService(filepath.Join(root, "state"), backend)
	if err := repositories.save(repo); err != nil {
		t.Fatal(err)
	}
	if err := repositories.save(work); err != nil {
		t.Fatal(err)
	}
	environment := core.Environment{Name: "dev", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:work"}, RuntimeRef: "test:dev", CreatedAt: time.Now().UTC()}
	// Unix sockets have a short path limit independent of checkout/temp names.
	sockets, err := os.MkdirTemp("", "haco-git-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockets) })
	broker := NewBroker(repositories, singleEnvironment{environment}, sockets)
	audit := &gitAudit{}
	capabilities, err := capabilityapp.New(gitPolicy{}, nil, audit, broker)
	if err != nil {
		t.Fatal(err)
	}
	broker.Capabilities = capabilities
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := broker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	if err := broker.Connect(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := "#!/bin/sh\nexec '" + strings.ReplaceAll(self, "'", "'\"'\"'") + "' -test.run=TestGitHelperProcess -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git-remote-haco"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("HACO_GIT_TEST_HELPER", "1")
	t.Setenv("HACO_GIT_TEST_SOCKET", broker.socket("dev"))
	testGit(t, workspace, "fetch", "origin")
	upstream := testCommit(t, seed, "upstream.txt", "pulled normally\n")
	testGit(t, seed, "push", "origin", "main")
	testGit(t, workspace, "pull", "--ff-only")
	if got := testGit(t, workspace, "rev-parse", "HEAD"); got != upstream {
		t.Fatalf("pull=%s", got)
	}
	if got := testGit(t, filepath.Join(backend.repos, "demo"), "rev-parse", "HEAD"); got != initial {
		t.Fatal("guest modified trusted worktree")
	}
	approved := testCommit(t, workspace, "work.txt", "approved work\n")
	push := func() (chan error, *bytes.Buffer) {
		output := new(bytes.Buffer)
		cmd := exec.Command("/usr/bin/git", "-C", workspace, "push", "origin", "main")
		cmd.Stdout = output
		cmd.Stderr = output
		done := make(chan error, 1)
		go func() { done <- cmd.Run() }()
		return done, output
	}
	waitProposal := func() Proposal {
		t.Helper()
		deadline := time.NewTimer(10 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-deadline.C:
				t.Fatal("push did not request approval")
			case <-tick.C:
				if pending := broker.Pending(); len(pending) == 1 {
					return pending[0]
				}
			}
		}
	}
	done, output := push()
	proposal := waitProposal()
	if proposal.Repository != "demo" || proposal.Ref != "refs/heads/main" || proposal.OldOID != upstream || proposal.NewOID != approved || proposal.Remote != repo.Remote {
		t.Fatalf("proposal=%+v", proposal)
	}
	if err := broker.Decide(proposal.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatalf("denied push succeeded: %s", output)
	}
	if got := testGit(t, remote, "rev-parse", "main"); got != upstream {
		t.Fatal("denial changed remote")
	}
	done, output = push()
	proposal = waitProposal()
	// Editing the branch during approval must not change the approved content.
	unpushed := testCommit(t, workspace, "later.txt", "retained unpushed work\n")
	if err := broker.Decide(proposal.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("approved push failed: %v %s", err, output)
	}
	if got := testGit(t, remote, "rev-parse", "main"); got != approved {
		t.Fatalf("remote=%s want=%s", got, approved)
	}
	if got := testGit(t, workspace, "rev-parse", "HEAD"); got != unpushed {
		t.Fatal("local unpushed work lost")
	}
	if err := broker.Decide(proposal.ID, true); err == nil {
		t.Fatal("approval replay succeeded")
	}
	for _, req := range []Request{{Operation: "list", Repository: "other"}, {Operation: "approve", Repository: "demo"}, {Operation: "push", Repository: "demo", Ref: "refs/heads/other", OldOID: approved, NewOID: unpushed, Pack: []byte("bad")}} {
		if _, err := UnixExchange(broker.socket("dev"))(ctx, req); err == nil {
			t.Fatalf("accepted %#v", req)
		}
	}
	// An arbitrary call to the general Capability provider has no prepared
	// operation context, even when it knows the reviewed operation identity.
	if _, err := broker.Execute(ctx, core.CapabilityRequest{Capability: Capability, Action: "push", Attributes: map[string]string{"operation_id": proposal.ID}}); err == nil {
		t.Fatal("unprepared provider call succeeded")
	}
	audit.mu.Lock()
	data, _ := json.Marshal(audit.events)
	audit.mu.Unlock()
	if bytes.Contains(data, []byte("approved work")) || bytes.Contains(data, []byte("PACK")) {
		t.Fatal("audit contains transferred content")
	}
}
