package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("Git transfer exceeds PoC size limit")
	}
	return b.Buffer.Write(p)
}

// Agent executes one controller-selected operation. It has no server socket
// and cannot create Environments, change Policy or approve a push.
func Agent(ctx context.Context, input io.Reader, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var req AgentRequest
	decoder := json.NewDecoder(io.LimitReader(input, MaxMessage+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return fmt.Errorf("invalid trusted Git request")
	}
	result, err := RunAgent(ctx, req, RepositoryRoot, WorkspaceRoot)
	if err != nil {
		result = Response{Error: err.Error()}
	}
	return json.NewEncoder(output).Encode(result)
}

func RunAgent(ctx context.Context, req AgentRequest, repos, workspaces string) (Response, error) {
	if !ValidID(req.Repository) || !ValidBranch(req.Branch) || ValidateRemote(req.Remote) != nil || len(req.Pack) > MaxPack {
		return Response{}, fmt.Errorf("invalid trusted Git request")
	}
	dir := filepath.Join(repos, req.Repository)
	ref := "refs/heads/" + req.Branch
	tracking := "refs/remotes/origin/" + req.Branch
	git := func(stdin []byte, args ...string) ([]byte, error) { return trustedGit(ctx, dir, stdin, args...) }
	switch req.Operation {
	case "clone":
		_, err := trustedGit(ctx, "", nil, "clone", "--template=", "--no-local", "--single-branch", "--no-tags", "--branch", req.Branch, "--", req.Remote, dir)
		return Response{}, err
	case "workspace":
		if !ValidID(req.Workspace) {
			return Response{}, fmt.Errorf("invalid Workspace")
		}
		workspace := filepath.Join(workspaces, req.Workspace)
		// This is a fresh owned copy, before any Environment can write it. Keep
		// only local Git data and the non-authorizing helper URL in its config.
		config := "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n[remote \"origin\"]\n\turl = haco://" + req.Repository + "\n\tfetch = +" + ref + ":" + tracking + "\n[branch \"" + req.Branch + "\"]\n\tremote = origin\n\tmerge = " + ref + "\n"
		info, err := os.Lstat(filepath.Join(workspace, ".git"))
		if err != nil || !info.IsDir() {
			return Response{}, fmt.Errorf("Workspace must have its own .git directory")
		}
		return Response{}, os.WriteFile(filepath.Join(workspace, ".git", "config"), []byte(config), 0600)
	case "list", "fetch", "prepare", "push":
	default:
		return Response{}, fmt.Errorf("unsupported trusted Git operation")
	}
	info, err := os.Lstat(filepath.Join(dir, ".git"))
	if err != nil || !info.IsDir() {
		return Response{}, fmt.Errorf("trusted repository is unavailable")
	}
	if req.Operation == "push" {
		if !ValidOID(req.OldOID) || !ValidOID(req.NewOID) {
			return Response{}, fmt.Errorf("invalid approved OIDs")
		}
		if _, err := git(nil, "merge-base", "--is-ancestor", req.OldOID, req.NewOID); err != nil {
			return Response{}, fmt.Errorf("non-fast-forward push is unsupported")
		}
		// Both ends are fixed. The remote atomically compares the approved old
		// OID; moving a local branch during approval cannot change the payload.
		if _, err := git(nil, "push", "--porcelain", "--no-verify", "--force-with-lease="+ref+":"+req.OldOID, "--", req.Remote, req.NewOID+":"+ref); err != nil {
			return Response{}, fmt.Errorf("approved push failed or remote changed; fetch before retrying")
		}
		return Response{OID: req.NewOID, Ref: ref}, nil
	}
	if _, err := git(nil, "fetch", "--no-tags", "--no-recurse-submodules", "--", req.Remote, "+"+ref+":"+tracking); err != nil {
		return Response{}, fmt.Errorf("trusted remote fetch failed; check registration and Host authentication")
	}
	value, err := git(nil, "rev-parse", "--verify", tracking+"^{commit}")
	if err != nil {
		return Response{}, err
	}
	oid := strings.TrimSpace(string(value))
	if !ValidOID(oid) {
		return Response{}, fmt.Errorf("remote did not return a SHA-1 commit")
	}
	result := Response{OID: oid, Ref: ref}
	if req.Operation == "list" {
		return result, nil
	}
	if req.Operation == "fetch" {
		if req.NewOID != oid {
			return Response{}, fmt.Errorf("remote changed since listing; fetch again")
		}
		result.Pack, err = git([]byte(oid+"\n"), "pack-objects", "--stdout", "--revs")
		return result, err
	}
	if !ValidOID(req.NewOID) || req.OldOID != oid || len(req.Pack) == 0 {
		return Response{}, fmt.Errorf("push does not match the listed remote commit")
	}
	// The pack contains only Git objects, never a guest .git directory/config.
	if _, err := git(req.Pack, "index-pack", "--stdin", "--strict", "--max-input-size=33554432"); err != nil {
		return Response{}, fmt.Errorf("invalid Git object pack")
	}
	if _, err := git(nil, "cat-file", "-e", req.NewOID+"^{commit}"); err != nil {
		return Response{}, fmt.Errorf("new OID is not an available commit")
	}
	if _, err := git(nil, "merge-base", "--is-ancestor", oid, req.NewOID); err != nil {
		return Response{}, fmt.Errorf("non-fast-forward push is unsupported")
	}
	summary, err := git(nil, "diff", "--no-ext-diff", "--no-textconv", "--stat", oid, req.NewOID, "--")
	if err != nil {
		return Response{}, err
	}
	if len(summary) > 8192 {
		summary = summary[:8192]
	}
	result.Summary = string(summary)
	return result, nil
}

func trustedGit(ctx context.Context, dir string, input []byte, args ...string) ([]byte, error) {
	options := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.attributesFile=/dev/null", "-c", "core.pager=cat", "-c", "color.ui=false", "-c", "credential.helper=", "-c", "credential.helper=!/usr/bin/gh auth git-credential", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.file.allow=always", "-c", "fetch.fsckObjects=true", "-c", "transfer.fsckObjects=true", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	if dir != "" {
		options = append(options, "-C", dir)
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/git", append(options, args...)...)
	// Only the trusted Host's gh store is consulted. No caller environment,
	// global Git configuration, replace refs, hooks or external diff is loaded.
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/root", "LANG=C.UTF-8", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1"}
	cmd.Stdin = bytes.NewReader(input)
	var out cappedBuffer
	out.limit = MaxPack
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("trusted Git %s failed", args[0])
	}
	return out.Bytes(), nil
}
