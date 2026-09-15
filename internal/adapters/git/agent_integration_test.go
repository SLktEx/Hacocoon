//go:build linux

package gitadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These are adapter contracts against local, credential-free Git repositories.
// Policy and ownership approval remain covered by internal/git integration tests.
func TestAgentAndHelperTransferObjectsAndPinPushTarget(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git := func(dir string, args ...string) []byte {
		t.Helper()
		cmd := exec.Command("/usr/bin/git", append([]string{"-C", dir}, args...)...)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid"}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture Git failed: %v: %s", err, out)
		}
		return out
	}
	remote, seed, repos, workspaces := filepath.Join(root, "remote.git"), filepath.Join(root, "seed"), filepath.Join(root, "repos"), filepath.Join(root, "workspaces")
	for _, dir := range []string{remote, seed, repos, workspaces} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	git(remote, "init", "--bare", "--initial-branch=main")
	git(seed, "init", "--initial-branch=main")
	commit := func(dir, body string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "work.txt"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		git(dir, "add", "--", "work.txt")
		git(dir, "commit", "-m", "fixture work")
		return strings.TrimSpace(string(git(dir, "rev-parse", "HEAD")))
	}
	initial := commit(seed, "initial")
	git(seed, "push", "file://"+remote, "main")
	base := AgentRequest{Repository: "source", Workspace: "work", Remote: "file://" + remote, Branch: "main"}
	run := func(req AgentRequest) (Response, error) { return RunAgent(ctx, req, repos, workspaces) }
	clone := base
	clone.Operation = "clone"
	if _, err := run(clone); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(workspaces, "work")
	git(root, "clone", "--no-local", filepath.Join(repos, "source"), work)
	configure := base
	configure.Operation = "workspace"
	if _, err := run(configure); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(git(work, "config", "--get", "remote.origin.url"))); got != "haco://source" {
		t.Fatal("Workspace leaked remote", got)
	}
	t.Chdir(work)
	exchange := func(ctx context.Context, req Request) (Response, error) {
		if req.Repository != base.Repository {
			t.Fatal("helper substituted repository")
		}
		op := base
		op.Operation, op.Heads, op.Ref, op.OldOID, op.NewOID, op.Pack = req.Operation, req.Heads, req.Ref, req.OldOID, req.NewOID, req.Pack
		if req.Operation == "push" {
			op.Operation = "prepare"
			prepared, err := run(op)
			if err != nil {
				return Response{}, err
			}
			if prepared.Ref != req.Ref || prepared.OID != req.OldOID || !strings.Contains(prepared.Summary, "work.txt") {
				t.Fatal("incorrect prepared review", prepared)
			}
			op.Operation, op.Pack = "push", nil
		}
		return run(op)
	}
	for _, body := range []string{"first", "second"} {
		next := commit(work, body)
		var out, diagnostic bytes.Buffer
		input := "capabilities\noption progress false\noption unsupported value\nlist\nfetch " + initial + " refs/heads/main\n\npush HEAD:refs/heads/topic\n\n\n"
		if err := Helper(ctx, []string{"origin", "haco://source"}, strings.NewReader(input), &out, &diagnostic, exchange); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "ok refs/heads/topic\n") || !strings.Contains(diagnostic.String(), "Policy/approval") {
			t.Fatal("missing helper receipt", out.String())
		}
		if got := strings.TrimSpace(string(git(remote, "rev-parse", "refs/heads/topic"))); got != next {
			t.Fatal("push changed wrong commit", got)
		}
	}
	// A receiver cannot bypass the preparation path by changing object bytes.
	bad := base
	bad.Operation, bad.Ref, bad.OldOID, bad.NewOID, bad.Pack = "push", "refs/heads/topic", initial, initial, []byte("another pack")
	if _, err := run(bad); err == nil {
		t.Fatal("accepted replacement pack after approval")
	}
}

func TestTrustedWireRejectsUnknownFieldsAndInvalidRouting(t *testing.T) {
	for _, body := range []string{"{", `{"command":"sh"}`} {
		var out bytes.Buffer
		if err := Agent(context.Background(), strings.NewReader(body), &out); err == nil || out.Len() != 0 {
			t.Fatal("malformed wire request executed", out.String(), err)
		}
	}
	var out bytes.Buffer
	if err := Agent(context.Background(), strings.NewReader(`{"repository":"../other"}`), &out); err != nil {
		t.Fatal(err)
	}
	var response Response
	if json.Unmarshal(out.Bytes(), &response) != nil || response.Error == "" || response.OID != "" {
		t.Fatal("invalid operation got success", out.String())
	}
	for _, remote := range []string{"https://github.com/owner/repo.git", "file:///tmp/local.git"} {
		if ValidateRemote(remote) != nil || !ValidWorkspaceRouting(remote, "feature/work") {
			t.Fatal("valid routing rejected", remote)
		}
	}
	if !ValidWorkspaceRouting("", "") {
		t.Fatal("offline Workspace rejected")
	}
	for _, remote := range []string{"https://user:secret@github.com/o/r", "https://github.com/o/r?token=x", "https://github.com/o/r#x", "https://github.com/o/r%2fother", "https://evil.invalid/o/r", "file:///tmp/../other", "--upload-pack=sh", "https://github.com/o/r\n", "%"} {
		if ValidateRemote(remote) == nil || ValidWorkspaceRouting(remote, "main") {
			t.Fatalf("invalid remote accepted: %q", remote)
		}
	}
	for _, branch := range []string{"", "-option", "../main", "a//b", "a/", "a\n"} {
		if ValidBranch(branch) || ValidWorkspaceRouting("file:///tmp/repo", branch) {
			t.Fatalf("invalid branch accepted: %q", branch)
		}
	}
}

func TestAgentRefusesMalformedOperationsBeforeExternalMutation(t *testing.T) {
	root := t.TempDir()
	base := AgentRequest{Repository: "repo", Remote: "file:///nonexistent", Branch: "main", Operation: "list"}
	for _, mutate := range []func(*AgentRequest){
		func(r *AgentRequest) { r.Repository = "../other" },
		func(r *AgentRequest) { r.Branch = "-option" },
		func(r *AgentRequest) { r.Remote = "https://user:secret@github.com/o/r" },
		func(r *AgentRequest) { r.Heads = []Head{{Ref: "refs/heads/main", OID: strings.Repeat("a", 40)}} },
		func(r *AgentRequest) { r.Ref = "refs/heads/main" },
		func(r *AgentRequest) { r.Operation = "fetch" },
		func(r *AgentRequest) {
			r.Operation = "fetch"
			r.Heads = []Head{{Ref: "refs/heads/main", OID: strings.Repeat("a", 40)}}
			r.NewOID = strings.Repeat("b", 40)
		},
		func(r *AgentRequest) { r.Operation = "workspace"; r.Workspace = "../other" },
		func(r *AgentRequest) { r.Operation = "workspace"; r.Workspace = "missing" },
		func(r *AgentRequest) { r.Operation = "execute" },
		func(r *AgentRequest) {}, // valid list with no trusted repository
	} {
		req := base
		mutate(&req)
		if got, err := RunAgent(context.Background(), req, root, root); err == nil || got.OID != "" {
			t.Fatal("unsafe or unavailable operation accepted", req.Operation, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected operation created material", entries, err)
	}
}

func TestUnixExchangeRefusesMalformedAndFailedBrokerResponses(t *testing.T) {
	dir, err := os.MkdirTemp("", "haco-wire-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	socket := filepath.Join(dir, "git.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/git" {
			t.Error("wrong wire route")
		}
		var req Request
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			t.Error("invalid request encoding")
		}
		var body string
		switch req.Operation {
		case "unknown-field":
			body = `{"command":"sh"}`
		case "invalid-json":
			body = `{`
		case "refused":
			w.WriteHeader(http.StatusForbidden)
			body = `{"error":"denied"}`
		case "failed":
			body = `{"error":"unavailable"}`
		default:
			body = `{"ref":"refs/heads/main"}`
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Error(err)
		}
	})}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-served; !errors.Is(err, http.ErrServerClosed) {
			t.Error("unexpected broker termination", err)
		}
	})
	exchange := UnixExchange(socket)
	for _, op := range []string{"valid", "unknown-field", "invalid-json", "refused", "failed"} {
		response, err := exchange(context.Background(), Request{Operation: op})
		if op == "valid" {
			if err != nil || response.Ref != "refs/heads/main" {
				t.Fatal(response, err)
			}
		} else if err == nil {
			t.Fatal("invalid broker response accepted", op)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := exchange(ctx, Request{}); err == nil {
		t.Fatal("canceled request accepted")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := exchange(context.Background(), Request{}); err == nil || errors.Is(err, context.Canceled) {
		t.Fatal("unavailable broker accepted", err)
	}
}
