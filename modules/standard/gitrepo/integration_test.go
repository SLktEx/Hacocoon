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
	"github.com/SLktEx/Hacocoon/internal/review"
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
	testGit(t, seed, "switch", "-c", "feature/topic.v2")
	feature := testCommit(t, seed, "feature.txt", "separate branch\n")
	testGit(t, seed, "push", "origin", "feature/topic.v2")
	testGit(t, seed, "switch", "-c", "feature/independent", initial)
	independent := testCommit(t, seed, "independent.txt", "another branch in one fetch batch\n")
	testGit(t, seed, "push", "origin", "feature/independent")
	testGit(t, seed, "switch", "main")
	backend := localBackend{repos: filepath.Join(root, "repos"), workspaces: filepath.Join(root, "workspaces")}
	repo := Object{Kind: "repo", ID: "demo", Repository: "demo", Remote: "file://" + remote, Branch: "main", NativeRef: "test-volume", Owner: strings.Repeat("a", 32), State: "ready"}
	work := repo
	work.Kind = "work"
	work.ID = "work"
	work.Owner = strings.Repeat("b", 32)
	if _, err := backend.RunGit(context.Background(), AgentRequest{Operation: "clone", Repository: repo.ID, Remote: repo.Remote, Branch: repo.Branch}); err != nil {
		t.Fatal(err)
	}
	if got := testGit(t, filepath.Join(backend.repos, "demo"), "rev-parse", "refs/remotes/origin/feature/topic.v2"); got != feature {
		t.Fatal("Host clone omitted a non-default branch")
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
	identity, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	broker := NewBroker(repositories, &identityEnvironmentStore{environment: environment, identity: identity}, sockets)
	policyPath := filepath.Join(root, "saved-policy.json")
	initialPolicy := `{"default":"allow","rules":[{"capability":"git.repository","action":"push","environment":"*","resource":"*","attributes":{"repository":"*","remote":"*","target_ref":"*","old_oid":"*","new_oid":"*","operation_id":"*","update_kind":"fast-forward"},"decision":"require-approval"}]}`
	if err := os.WriteFile(policyPath, []byte(initialPolicy), 0600); err != nil {
		t.Fatal(err)
	}
	audit := &gitAudit{}
	capabilities, err := capabilityapp.New(capabilityapp.NewFilePolicyEvaluator(policyPath), nil, audit, broker)
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
	testGit(t, seed, "switch", "feature/topic.v2")
	feature = testCommit(t, seed, "private.txt", "must not be fetched under a per-ref deny\n")
	testGit(t, seed, "push", "origin", "feature/topic.v2")
	testGit(t, seed, "switch", "main")
	// A prior read rule limited to main cannot silently grant all-heads reads.
	limitedRead := `{"default":"deny","rules":[{"capability":"git.repository","action":"fetch","environment":"*","resource":"*","attributes":{"repository":"*","remote":"*","target_ref":"refs/heads/main","old_oid":"*","new_oid":"*","operation_id":"*"},"decision":"allow"}]}`
	if err := os.WriteFile(policyPath, []byte(limitedRead), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := UnixExchange(broker.socket("dev"))(ctx, Request{Operation: "list", Repository: "demo"}); err == nil {
		t.Fatal("one-branch read rule authorized all heads")
	}
	deniedHead := `{"default":"allow","rules":[{"capability":"git.repository","action":"fetch","environment":"*","resource":"*","attributes":{"repository":"*","remote":"*","target_ref":"refs/heads/feature/topic.v2","old_oid":"*","new_oid":"*","operation_id":"*"},"decision":"deny"}]}`
	if err := os.WriteFile(policyPath, []byte(deniedHead), 0600); err != nil {
		t.Fatal(err)
	}
	for _, request := range []Request{{Operation: "list", Repository: "demo"}, {Operation: "fetch", Repository: "demo", Heads: []Head{{Ref: "refs/heads/feature/topic.v2", OID: feature}}}} {
		if _, err := UnixExchange(broker.socket("dev"))(ctx, request); err == nil {
			t.Fatal("broad reads bypassed exact-ref deny")
		}
	}
	if _, err := trustedGit(ctx, filepath.Join(backend.repos, "demo"), nil, "cat-file", "-e", feature); err == nil {
		t.Fatal("denied branch objects reached Host before authorization")
	}
	if err := os.WriteFile(policyPath, []byte(initialPolicy), 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, workspace, "fetch", "origin")
	if got := testGit(t, workspace, "rev-parse", "refs/remotes/origin/feature/topic.v2"); got != feature {
		t.Fatal("ordinary fetch omitted alternate branch")
	}
	if got := testGit(t, workspace, "rev-parse", "refs/remotes/origin/feature/independent"); got != independent {
		t.Fatal("ordinary batch fetch omitted independent history")
	}
	testGit(t, workspace, "switch", "--track", "origin/feature/topic.v2")
	if got := testGit(t, workspace, "rev-parse", "HEAD"); got != feature {
		t.Fatal("cannot switch to fetched branch")
	}
	testGit(t, workspace, "switch", "main")
	// A newly advertised branch after clone must pass through the helper too.
	testGit(t, seed, "branch", "second", feature)
	testGit(t, seed, "push", "origin", "second")
	testGit(t, workspace, "fetch", "origin")
	if got := testGit(t, workspace, "rev-parse", "refs/remotes/origin/second"); got != feature {
		t.Fatal("new upstream head omitted")
	}
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
	push := func(refs ...string) (chan error, *bytes.Buffer) {
		if len(refs) == 0 {
			refs = []string{"main"}
		}
		output := new(bytes.Buffer)
		cmd := exec.Command("/usr/bin/git", append([]string{"-C", workspace, "push", "origin"}, refs...)...)
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
	// Reusable Policy must approve future commits on this registered branch,
	// while every execution still carries the exact prepared old/new OIDs.
	resetPolicy := func() {
		t.Helper()
		if err := os.WriteFile(policyPath, []byte(`{"default":"require-approval","rules":[{"capability":"git.repository","action":"fetch","environment":"*","resource":"*","attributes":{"repository":"*","remote":"*","target_ref":"*","old_oid":"*","new_oid":"*","operation_id":"*"},"decision":"allow"}]}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	resetPolicy()
	finishPush := func(done <-chan error, wantSuccess bool) {
		t.Helper()
		select {
		case err := <-done:
			if (err == nil) != wantSuccess {
				t.Fatalf("push success=%v: %v", wantSuccess, err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("saved-policy push did not finish")
		}
	}
	done, _ = push()
	proposal = waitProposal()
	if proposal.SavedScope == nil || proposal.SavedScope.Attributes["target_ref"] != "refs/heads/main" || proposal.SavedScope.Attributes["update_kind"] != "fast-forward" ||
		proposal.SavedScope.Attributes["new_oid"] != "*" || proposal.SavedScope.Attributes["operation_id"] != "*" {
		t.Fatalf("incorrect reusable scope: %+v", proposal.SavedScope)
	}
	// Pending snapshots must not let a display client rewrite stored scope.
	proposal.SavedScope.Attributes["target_ref"] = "refs/heads/other"
	reviews := review.New(broker)
	reviewPending, err := reviews.Pending(ctx)
	if err != nil || len(reviewPending) != 1 || reviewPending[0].RequestID != proposal.RequestID {
		t.Fatalf("common review lost Git request: %+v %v", reviewPending, err)
	}
	reviewPending[0].CapabilityRequest.Attributes["target_ref"] = "refs/heads/other"
	canceledReview, cancelReview := context.WithCancel(ctx)
	cancelReview()
	if _, err := broker.DecideWithDecision(canceledReview, proposal.ID, capabilityapp.ApprovalDecision{Approved: true}); err == nil || len(broker.Pending()) != 1 {
		t.Fatal("canceled Git review consumed approval")
	}
	result, err := reviews.Decide(ctx, proposal.RequestID, capabilityapp.ApprovalDecision{Approved: true, Save: capabilityapp.AllowEnvironment})
	if err != nil || result.SavedChoice != string(capabilityapp.AllowEnvironment) || proposal.RequestID == "" || result.RequestID != proposal.RequestID {
		t.Fatalf("saved result: %+v %v", result, err)
	}
	finishPush(done, true)
	next := testCommit(t, workspace, "next.txt", "next approved commit\n")
	done, _ = push()
	finishPush(done, true)
	if got := testGit(t, remote, "rev-parse", "main"); got != next {
		t.Fatal("saved scope did not push next exact commit")
	}
	contents, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved capabilityapp.PolicyFile
	if err := json.Unmarshal(contents, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.SavedDecisions) != 1 || saved.SavedDecisions[0].Attributes["target_ref"] != "refs/heads/main" || saved.SavedDecisions[0].Attributes["old_oid"] != "*" {
		t.Fatal("saved scope changed or pinned a commit")
	}

	// A branch allow is not permission to rewrite its history.
	force := exec.Command("/usr/bin/git", "-C", workspace, "push", "--force", "origin", initial+":refs/heads/main")
	if err := force.Run(); err == nil {
		t.Fatal("saved allow authorized a history rewrite")
	}
	if got := testGit(t, remote, "rev-parse", "main"); got != next {
		t.Fatal("force request changed remote")
	}
	if _, err := broker.SavedApprovalScope(ctx, core.CapabilityRequest{Capability: Capability, Action: "push", Attributes: map[string]string{"operation_id": proposal.ID}}); err == nil {
		t.Fatal("unprepared caller obtained reusable scope")
	}

	deletion := exec.Command("/usr/bin/git", "-C", workspace, "push", "--delete", "origin", "main")
	if err := deletion.Run(); err == nil {
		t.Fatal("saved allow authorized branch deletion")
	}
	if got := testGit(t, remote, "rev-parse", "main"); got != next {
		t.Fatal("deletion changed remote")
	}

	// An explicit saved ask still requests a separate answer every time.
	resetPolicy()
	testCommit(t, workspace, "ask.txt", "ask\n")
	done, _ = push()
	proposal = waitProposal()
	result, err = broker.DecideWithDecision(ctx, proposal.ID, capabilityapp.ApprovalDecision{Approved: true, Save: capabilityapp.AskEnvironment})
	if err != nil || result.SavedChoice != string(capabilityapp.AskEnvironment) {
		t.Fatalf("save ask: %+v %v", result, err)
	}
	finishPush(done, true)
	testCommit(t, workspace, "deny.txt", "denied\n")
	done, _ = push()
	proposal = waitProposal()
	result, err = broker.DecideWithDecision(ctx, proposal.ID, capabilityapp.ApprovalDecision{Save: capabilityapp.DenyEnvironment})
	if err != nil || result.SavedChoice != string(capabilityapp.DenyEnvironment) {
		t.Fatalf("save deny: %+v %v", result, err)
	}
	finishPush(done, false)
	done, _ = push()
	finishPush(done, false)
	if len(broker.Pending()) != 0 {
		t.Fatal("saved deny prompted")
	}

	audit.mu.Lock()
	events := append([]core.CapabilityAuditEvent(nil), audit.events...)
	audit.mu.Unlock()
	foundSaved := false
	for _, event := range events {
		if event.Type == "policy-saved" {
			foundSaved = true
			if event.Attributes["new_oid"] == "" || event.SavedScope == nil || event.SavedScope.Attributes["new_oid"] != "*" || event.SavedScope.Attributes["target_ref"] != "refs/heads/main" {
				t.Fatal("audit confused execution with saved authority")
			}
		}
	}
	if !foundSaved {
		t.Fatal("saved scope was not audited")
	}
	data, _ := json.Marshal(events)
	if bytes.Contains(data, []byte("approved work")) || bytes.Contains(data, []byte("PACK")) {
		t.Fatal("audit contains transferred content")
	}

	t.Run("development branch creation and updates retain separate authority", func(t *testing.T) {
		resetPolicy()
		mainBefore := testGit(t, remote, "rev-parse", "main")
		testGit(t, workspace, "checkout", "-b", "feature/work")
		first := testCommit(t, workspace, "feature.txt", "first feature\n")
		done, output := push("feature/work")
		proposal := waitProposal()
		if proposal.Ref != "refs/heads/feature/work" || proposal.OldOID != ZeroOID || proposal.NewOID != first || proposal.SavedScope == nil || proposal.SavedScope.Attributes["update_kind"] != "create" {
			t.Fatalf("creation proposal: %+v", proposal)
		}
		if err := broker.Decide(proposal.ID, false); err != nil {
			t.Fatal(err)
		}
		finishPush(done, false)
		if got := testGit(t, remote, "for-each-ref", "--format=%(refname)", "refs/heads/feature/work"); got != "" {
			t.Fatal("denied creation wrote a remote ref")
		}
		done, output = push("feature/work")
		proposal = waitProposal()
		later := testCommit(t, workspace, "later-feature.txt", "not yet approved\n")
		if _, err := broker.DecideWithDecision(ctx, proposal.ID, capabilityapp.ApprovalDecision{Approved: true, Save: capabilityapp.AllowEnvironment}); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("create: %v %s", err, output)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("creation timed out")
		}
		if got := testGit(t, remote, "rev-parse", "feature/work"); got != first {
			t.Fatal("creation did not pin the approved commit")
		}
		if err := broker.Decide(proposal.ID, true); err == nil {
			t.Fatal("creation decision replayed")
		}
		// Saving create does not save update. The later commit needs a new answer.
		done, _ = push("feature/work")
		proposal = waitProposal()
		if proposal.OldOID != first || proposal.NewOID != later || proposal.SavedScope.Attributes["update_kind"] != "fast-forward" {
			t.Fatalf("update proposal: %+v", proposal)
		}
		if err := broker.Decide(proposal.ID, true); err != nil {
			t.Fatal(err)
		}
		finishPush(done, true)
		if got := testGit(t, remote, "rev-parse", "feature/work"); got != later {
			t.Fatal("update lost approved commit")
		}
		// Fetching all refs or saving a feature decision never grants main push.
		done, _ = push("HEAD:refs/heads/main")
		proposal = waitProposal()
		if proposal.Ref != "refs/heads/main" {
			t.Fatalf("main proposal: %+v", proposal)
		}
		if err := broker.Decide(proposal.ID, false); err != nil {
			t.Fatal(err)
		}
		finishPush(done, false)
		if got := testGit(t, remote, "rev-parse", "main"); got != mainBefore {
			t.Fatal("feature authority changed main")
		}

		// Both divergent and identical competing creations must fail closed.
		for _, identical := range []bool{false, true} {
			ref := fmt.Sprintf("refs/heads/race-%v", identical)
			done, _ = push("HEAD:" + ref)
			proposal = waitProposal()
			competitor := mainBefore
			if identical {
				competitor = proposal.NewOID
			}
			testGit(t, remote, "update-ref", ref, competitor)
			if err := broker.Decide(proposal.ID, true); err != nil {
				t.Fatal(err)
			}
			finishPush(done, false)
			if got := testGit(t, remote, "rev-parse", ref); got != competitor {
				t.Fatal("overwrote competing creation")
			}
		}
		for _, refs := range [][]string{{"HEAD:refs/heads/batch-a", "HEAD:refs/heads/batch-b"}, {"+HEAD:refs/heads/forced-new"}, {":refs/heads/feature/work"}} {
			done, _ = push(refs...)
			finishPush(done, false)
			if len(broker.Pending()) != 0 {
				t.Fatal("unsupported push requested approval")
			}
		}
	})
}
