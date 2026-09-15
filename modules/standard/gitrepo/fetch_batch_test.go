package gitrepo

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchBatchIndexesIndependentPacksBeyondSinglePackLimit(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git is required")
	}
	root := t.TempDir()
	remote, seed, repos, guest := filepath.Join(root, "remote"), filepath.Join(root, "seed"), filepath.Join(root, "repos"), filepath.Join(root, "guest")
	for _, dir := range []string{remote, seed, repos, guest} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=main")
	testGit(t, seed, "init", "--initial-branch=main")
	testGit(t, guest, "init", "--initial-branch=main")
	var heads []Head
	const blobSize = 17 << 20
	for _, branch := range []string{"main", "topic"} {
		if branch != "main" {
			testGit(t, seed, "switch", "--orphan", branch)
		}
		data := make([]byte, blobSize)
		if _, err := rand.Read(data); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(seed, "payload.bin"), data, 0600); err != nil {
			t.Fatal(err)
		}
		testGit(t, seed, "add", "payload.bin")
		testGit(t, seed, "commit", "-m", branch)
		oid := testGit(t, seed, "rev-parse", "HEAD")
		heads = append(heads, Head{Ref: "refs/heads/" + branch, OID: oid})
		testGit(t, seed, "push", "file://"+remote, branch)
	}
	ctx := context.Background()
	agent := AgentRequest{Operation: "clone", Repository: "demo", Remote: "file://" + remote, Branch: "main"}
	if _, err := RunAgent(ctx, agent, repos, ""); err != nil {
		t.Fatal(err)
	}
	t.Chdir(guest)
	total, calls := 0, 0
	exchange := func(ctx context.Context, request Request) (Response, error) {
		next := agent
		next.Operation, next.Heads, next.Haves = request.Operation, request.Heads, request.Haves
		response, err := RunAgent(ctx, next, repos, "")
		if request.Operation == "fetch" {
			if len(request.Heads) != 1 {
				t.Fatal("fetch combined separately authorized heads")
			}
			if len(response.Pack) <= 0 || len(response.Pack) > MaxPack {
				t.Fatal("invalid per-head pack", len(response.Pack), err)
			}
			total += len(response.Pack)
			calls++
		}
		return response, err
	}
	input := "list\n"
	for _, head := range heads {
		input += fmt.Sprintf("fetch %s %s\n", head.OID, head.Ref)
	}
	input += "\n\n"
	var output bytes.Buffer
	if err := Helper(ctx, []string{"origin", "haco://demo"}, strings.NewReader(input), &output, &output, exchange); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || total <= MaxPack {
		t.Fatal("fixture did not exceed old aggregate bound", calls, total)
	}
	for _, head := range heads {
		if got := testGit(t, guest, "cat-file", "-s", head.OID+":payload.bin"); got != fmt.Sprint(blobSize) {
			t.Fatal("missing fetched content", got)
		}
	}
	t.Logf("two separately indexed packs: %d total bytes; each below %d bytes", total, MaxPack)
}
