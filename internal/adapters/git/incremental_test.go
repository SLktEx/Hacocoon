package gitadapter

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitOutputLimitAppliesToPipeCopy(t *testing.T) {
	var output cappedBuffer
	output.limit = 8
	// exec.Cmd copies a pipe reader into this writer. Embedding bytes.Buffer
	// exposed ReadFrom and let io.Copy bypass the bounded Write method.
	_, err := io.Copy(&output, struct{ io.Reader }{strings.NewReader("more than eight bytes")})
	if err == nil || len(output.Bytes()) > output.limit {
		t.Fatal("pipe copy bypassed output limit", err, len(output.Bytes()))
	}
}

func TestHistoryHintsAreFetchOnlyAndBounded(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	for _, tc := range []struct {
		op    string
		haves []string
		valid bool
	}{
		{"fetch", nil, true}, {"fetch", []string{a, b}, true},
		{"push", []string{a}, false}, {"prepare", []string{a}, false},
		{"list", []string{a}, false}, {"fetch", []string{a, a}, false},
		{"fetch", []string{ZeroOID}, false}, {"fetch", []string{"--all"}, false},
		{"fetch", make([]string, maxHaves+1), false},
	} {
		if got := ValidHaves(tc.op, tc.haves); got != tc.valid {
			t.Fatalf("%s %v: valid=%v", tc.op, tc.haves, got)
		}
		if !tc.valid {
			if _, err := RunAgent(context.Background(), AgentRequest{Operation: tc.op, Haves: tc.haves}, "", ""); err == nil {
				t.Fatal("agent accepted invalid history hints")
			}
		}
	}
}

func TestOrdinaryIncrementalGitWithHistoryLargerThanPackLimit(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git is required")
	}
	root := t.TempDir()
	remote, seed, repos, guest := filepath.Join(root, "remote"), filepath.Join(root, "seed"), filepath.Join(root, "repos"), filepath.Join(root, "guest")
	for _, dir := range []string{remote, seed, repos} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=main")
	testGit(t, seed, "init", "--initial-branch=main")
	data := make([]byte, legacyPackLimit+1<<20)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "existing.bin"), data, 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, seed, "add", "existing.bin")
	testGit(t, seed, "commit", "-m", "existing history exceeds transport limit")
	old := testGit(t, seed, "rev-parse", "HEAD")
	testGit(t, seed, "push", "file://"+remote, "main")
	req := AgentRequest{Operation: "clone", Repository: "demo", Remote: "file://" + remote}
	ctx := context.Background()
	if _, err := RunAgent(ctx, req, repos, ""); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "clone", "--no-local", filepath.Join(repos, "demo"), guest)
	t.Chdir(guest)
	if _, err := helperGit(ctx, []byte(old+"\n"), "pack-objects", "--stdout", "--revs"); err == nil {
		t.Fatal("fixture did not reproduce the complete-history pack limit")
	}
	next := testCommit(t, seed, "small.txt", "remote update\n")
	testGit(t, seed, "push", "file://"+remote, "main")
	var received int64
	exchange := func(ctx context.Context, request Request) (Response, error) {
		agent := req
		agent.Operation, agent.Heads, agent.Haves = request.Operation, request.Heads, request.Haves
		agent.PackOutput = request.PackOutput
		response, err := RunAgent(ctx, agent, repos, "")
		if request.Operation == "fetch" {
			if len(request.Haves) == 0 {
				t.Fatal("helper omitted existing branch tips")
			}
			received += response.PackBytes
		}
		return response, err
	}
	var output bytes.Buffer
	if err := Helper(ctx, []string{"origin", "haco://demo"}, strings.NewReader("list\nfetch "+next+" refs/heads/main\n\n\n"), &output, &output, exchange); err != nil {
		t.Fatal(err)
	}
	if received == 0 || received > 64<<10 {
		t.Fatalf("small fetch transferred %d bytes", received)
	}
	if got := testGit(t, guest, "show", next+":small.txt"); got != "remote update" {
		t.Fatal(got)
	}
	testGit(t, guest, "reset", "--hard", next)
	local := testCommit(t, guest, "local.txt", "local update\n")
	var pack bytes.Buffer
	err := helperPushPack(ctx, local, next, &pack)
	if err != nil || pack.Len() > 64<<10 {
		t.Fatalf("incremental push: %d bytes, %v", pack.Len(), err)
	}
	prepared := req
	prepared.Operation, prepared.Ref, prepared.OldOID, prepared.NewOID, prepared.Pack = "prepare", "refs/heads/main", next, local, bytes.NewReader(pack.Bytes())
	if _, err := RunAgent(ctx, prepared, repos, ""); err != nil {
		t.Fatal(err)
	}
	if got := testGit(t, remote, "rev-parse", "refs/heads/main"); got != next {
		t.Fatal("preparation changed remote")
	}
	t.Logf("existing random data=%d bytes; fetch=%d bytes; prepared push=%d bytes", len(data), received, pack.Len())
	var operations []string
	var newBranchBytes int
	newBranchExchange := func(ctx context.Context, request Request) (Response, error) {
		operations = append(operations, request.Operation)
		if request.Operation != "push" {
			return exchange(ctx, request)
		}
		if request.OldOID != ZeroOID || request.Ref != "refs/heads/feature/new" || request.NewOID != local {
			t.Fatal("history reuse changed the proposed target", request.Ref, request.OldOID)
		}
		var packed bytes.Buffer
		if _, err := io.Copy(&packed, request.Pack); err != nil {
			return Response{}, err
		}
		newBranchBytes = packed.Len()
		request.Pack = bytes.NewReader(packed.Bytes())
		agent := req
		agent.Operation, agent.Ref, agent.OldOID, agent.NewOID, agent.Pack = "prepare", request.Ref, request.OldOID, request.NewOID, request.Pack
		return RunAgent(ctx, agent, repos, "") // No external write in this component check.
	}
	output.Reset()
	if err := Helper(ctx, []string{"origin", "haco://demo"}, strings.NewReader("list for-push\npush HEAD:refs/heads/feature/new\n\n\n"), &output, &output, newBranchExchange); err != nil {
		t.Fatal(err)
	}
	if strings.Join(operations, ",") != "list,fetch,push" || newBranchBytes == 0 || newBranchBytes > 64<<10 || !strings.Contains(output.String(), "ok refs/heads/feature/new") {
		t.Fatalf("new branch: operations=%v bytes=%d output=%s", operations, newBranchBytes, output.String())
	}
	if got := testGit(t, remote, "for-each-ref", "--format=%(refname)", "refs/heads/"); got != "refs/heads/main" {
		t.Fatal("new-branch preparation mutated the remote", got)
	}
	t.Logf("new-branch prepared pack=%d bytes; same expected-absent target retained", newBranchBytes)
}

func TestNewBranchRefReadFailureNeverReachesPush(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git is required")
	}
	guest := t.TempDir()
	testGit(t, guest, "init", "--initial-branch=main")
	base := testCommit(t, guest, "base.txt", "base\n")
	_ = testCommit(t, guest, "work.txt", "work\n")
	t.Chdir(guest)
	for _, failure := range []string{"denied", "moved", "wrong confirmation"} {
		t.Run(failure, func(t *testing.T) {
			var operations []string
			exchange := func(_ context.Context, request Request) (Response, error) {
				operations = append(operations, request.Operation)
				response := Response{Ref: "refs/heads/main", OID: base, Heads: []Head{{Ref: "refs/heads/main", OID: base}}}
				if request.Operation == "list" {
					return response, nil
				}
				if request.Operation != "fetch" || len(request.Heads) != 1 || request.Heads[0] != response.Heads[0] || len(request.Haves) != 1 || request.Haves[0] != base {
					t.Fatal("failed basis read reached another operation", request.Operation)
				}
				if failure == "wrong confirmation" {
					response.OID, response.PackBytes = strings.Repeat("a", 40), 4
					return response, nil
				}
				return Response{}, fmt.Errorf("basis read %s", failure)
			}
			var output bytes.Buffer
			err := Helper(context.Background(), []string{"origin", "haco://demo"}, strings.NewReader("list for-push\npush HEAD:refs/heads/new\n\n\n"), &output, &output, exchange)
			if err == nil || strings.Join(operations, ",") != "list,fetch" {
				t.Fatal("read failure did not stop before push", operations, err)
			}
		})
	}
}

func TestFetchExcludesOnlyAncestorsOfAuthorizedHead(t *testing.T) {
	head, ancestor, hidden, missing := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40), strings.Repeat("d", 40)
	var revisions string
	git := func(input []byte, args ...string) ([]byte, error) {
		switch args[0] {
		case "fetch":
			return nil, nil
		case "rev-parse":
			return []byte(head + "\n"), nil
		case "cat-file":
			return []byte("commit\n"), nil
		case "merge-base":
			if args[2] == ancestor && args[3] == head {
				return nil, nil
			}
			return nil, os.ErrNotExist
		case "pack-objects":
			revisions = string(input)
			return []byte("pack"), nil
		default:
			t.Fatalf("unexpected Git call %v", args)
			return nil, nil
		}
	}
	_, err := fetchHead(git, AgentRequest{Heads: []Head{{Ref: "refs/heads/main", OID: head}}, Haves: []string{hidden, missing, ancestor}}, func(input []byte) (int64, error) { revisions = string(input); return 4, nil })
	if err != nil || revisions != head+"\n^"+ancestor+"\n" {
		t.Fatalf("revisions=%q err=%v", revisions, err)
	}
}
