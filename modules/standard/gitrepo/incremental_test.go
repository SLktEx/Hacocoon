package gitrepo

import (
	"bytes"
	"context"
	"crypto/rand"
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
		if got := validHaves(tc.op, tc.haves); got != tc.valid {
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
	data := make([]byte, MaxPack+1<<20)
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
	req := AgentRequest{Operation: "clone", Repository: "demo", Remote: "file://" + remote, Branch: "main"}
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
	var received int
	exchange := func(ctx context.Context, request Request) (Response, error) {
		agent := req
		agent.Operation, agent.Heads, agent.Haves = request.Operation, request.Heads, request.Haves
		response, err := RunAgent(ctx, agent, repos, "")
		if request.Operation == "fetch" {
			if len(request.Haves) == 0 {
				t.Fatal("helper omitted existing branch tips")
			}
			received += len(response.Pack)
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
	pack, err := helperPushPack(ctx, local, next)
	if err != nil || len(pack) > 64<<10 {
		t.Fatalf("incremental push: %d bytes, %v", len(pack), err)
	}
	prepared := req
	prepared.Operation, prepared.Ref, prepared.OldOID, prepared.NewOID, prepared.Pack = "prepare", "refs/heads/main", next, local, pack
	if _, err := RunAgent(ctx, prepared, repos, ""); err != nil {
		t.Fatal(err)
	}
	if got := testGit(t, remote, "rev-parse", "refs/heads/main"); got != next {
		t.Fatal("preparation changed remote")
	}
	t.Logf("existing random data=%d bytes; fetch=%d bytes; prepared push=%d bytes", len(data), received, len(pack))
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
	_, err := fetchHead(git, AgentRequest{Heads: []Head{{Ref: "refs/heads/main", OID: head}}, Haves: []string{hidden, missing, ancestor}})
	if err != nil || revisions != head+"\n^"+ancestor+"\n" {
		t.Fatalf("revisions=%q err=%v", revisions, err)
	}
}
