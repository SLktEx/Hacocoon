package gitrepo

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustedFetchRejectsStaleDeletedAndUnlistedCommits(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git is required")
	}
	root := t.TempDir()
	remote, seed, repos := filepath.Join(root, "remote"), filepath.Join(root, "seed"), filepath.Join(root, "repos")
	for _, dir := range []string{remote, seed, repos} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=main")
	testGit(t, seed, "init", "--initial-branch=main")
	old := testCommit(t, seed, "file", "old\n")
	testGit(t, seed, "remote", "add", "origin", "file://"+remote)
	testGit(t, seed, "push", "origin", "main")
	testGit(t, seed, "branch", "topic")
	testGit(t, seed, "push", "origin", "topic")
	req := gitadapter.AgentRequest{Operation: "clone", Repository: "demo", Remote: "file://" + remote}
	if _, err := gitadapter.RunAgent(context.Background(), req, repos, ""); err != nil {
		t.Fatal(err)
	}
	req.Operation = "list"
	listed, err := gitadapter.RunAgent(context.Background(), req, repos, "")
	if err != nil || len(listed.Heads) != 2 {
		t.Fatal(listed, err)
	}
	req.Operation, req.Heads = "fetch", listed.Heads[:1]
	req.PackOutput = io.Discard
	if got, err := gitadapter.RunAgent(context.Background(), req, repos, ""); err != nil || got.PackBytes == 0 {
		t.Fatal("valid batch", err)
	}
	for _, heads := range [][]gitadapter.Head{{{Ref: "refs/heads/unlisted", OID: old}}, {{Ref: "refs/tags/topic", OID: old}}, {{Ref: "refs/heads/topic", OID: old}, {Ref: "refs/heads/topic", OID: old}}, {{Ref: "refs/heads/topic", OID: strings.Repeat("f", 40)}}} {
		req.Heads = heads
		if got, err := gitadapter.RunAgent(context.Background(), req, repos, ""); err == nil || got.PackBytes != 0 {
			t.Fatal("unreviewed commit escaped", heads, err)
		}
	}
	newOID := testCommit(t, seed, "file", "new\n")
	testGit(t, seed, "push", "origin", "main")
	req.Heads = []gitadapter.Head{{Ref: "refs/heads/main", OID: old}}
	if _, err := gitadapter.RunAgent(context.Background(), req, repos, ""); err == nil {
		t.Fatal("stale commit accepted")
	}
	testGit(t, seed, "push", "origin", "--delete", "topic")
	req.Heads = []gitadapter.Head{{Ref: "refs/heads/topic", OID: old}}
	if _, err := gitadapter.RunAgent(context.Background(), req, repos, ""); err == nil {
		t.Fatal("deleted branch remained fetchable from Host cache")
	}
	req.Heads = []gitadapter.Head{{Ref: "refs/heads/main", OID: newOID}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gitadapter.RunAgent(ctx, req, repos, ""); err == nil {
		t.Fatal("canceled fetch succeeded")
	}
}
