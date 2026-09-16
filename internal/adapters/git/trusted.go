package gitadapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *cappedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		return 0, fmt.Errorf("Git transfer exceeds PoC size limit")
	}
	return b.buffer.Write(p)
}

// Agent executes one controller-selected operation. It has no server socket
// and cannot create Environments, change Policy or approve a push.
func Agent(ctx context.Context, input io.Reader, output io.Writer) error {
	return ServeAgent(ctx, input, output, RepositoryRoot, WorkspaceRoot)
}

// ServeAgent completes one framed operation against controller-selected roots.
func ServeAgent(ctx context.Context, input io.Reader, output io.Writer, repos, workspaces string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := ReadAgentRequest(input)
	if err != nil {
		return err
	}
	stream := &transferWriter{target: output}
	req.PackOutput = stream
	result, err := RunAgent(ctx, req, repos, workspaces)
	if err != nil {
		result = Response{Error: err.Error()}
	}
	return finishResponse(stream, result)

}

func RunAgent(ctx context.Context, req AgentRequest, repos, workspaces string) (Response, error) {
	if req.Pack != nil && req.Operation != "prepare" {
		return Response{}, fmt.Errorf("pack is only valid for preparation")
	}
	if !ValidHaves(req.Operation, req.Haves) {
		return Response{}, fmt.Errorf("invalid Git history hints")
	}
	if !ValidID(req.Repository) || (req.Branch != "" && (req.Operation != "workspace" || !ValidBranch(req.Branch))) || ValidateRemote(req.Remote) != nil {
		return Response{}, fmt.Errorf("invalid trusted Git request")
	}
	dir := filepath.Join(repos, req.Repository)
	git := func(stdin []byte, args ...string) ([]byte, error) { return trustedGit(ctx, dir, stdin, args...) }
	if req.Operation != "fetch" && len(req.Heads) != 0 {
		return Response{}, fmt.Errorf("heads are only valid for fetch")
	}
	if req.Ref != "" && req.Operation != "prepare" && req.Operation != "push" && req.Operation != "observe" {
		return Response{}, fmt.Errorf("target ref is only valid for push preparation or execution")
	}
	if req.Operation == "fetch" {
		if _, err := ValidateHeads(req.Heads); err != nil || req.OldOID != "" || req.NewOID != "" || req.Pack != nil {
			return Response{}, fmt.Errorf("invalid fetch heads")
		}
	}
	switch req.Operation {
	case "clone":
		_, err := trustedGit(ctx, "", nil, "clone", "--template=", "--no-local", "--no-tags", "--no-checkout", "--", req.Remote, dir)
		return Response{}, err
	case "workspace":
		if !ValidID(req.Workspace) {
			return Response{}, fmt.Errorf("invalid Workspace")
		}
		workspace := filepath.Join(workspaces, req.Workspace)
		// Only a fresh copy of the trusted source reaches this operation. Imported
		// and restored guest data never runs authenticated Git or checkout here.
		info, err := os.Lstat(filepath.Join(workspace, ".git"))
		if err != nil || !info.IsDir() {
			return Response{}, fmt.Errorf("Workspace must have its own .git directory")
		}
		workGit := func(input []byte, args ...string) ([]byte, error) { return trustedGit(ctx, workspace, input, args...) }
		branch := req.Branch
		if branch == "" {
			listed, err := remoteHeads(workGit, req.Remote)
			if err != nil {
				return Response{}, err
			}
			branch = strings.TrimPrefix(listed.Ref, "refs/heads/")
			if !ValidBranch(branch) {
				return Response{}, fmt.Errorf("remote HEAD does not name an available branch; select a Workspace branch")
			}
		}
		ref := "refs/heads/" + branch
		// Refresh all tracking heads, including branches added after registration.
		if _, err := workGit(nil, "fetch", "--no-tags", "--no-recurse-submodules", "--prune", "--", req.Remote, "+refs/heads/*:refs/remotes/origin/*"); err != nil {
			return Response{}, err
		}
		if _, err := workGit(nil, "checkout", "--no-track", "-B", branch, "refs/remotes/origin/"+branch, "--"); err != nil {
			return Response{}, err
		}
		config := "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n[remote \"origin\"]\n\turl = haco://" + req.Repository + "\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n[branch " + strconv.Quote(branch) + "]\n\tremote = origin\n\tmerge = " + strconv.Quote(ref) + "\n"
		return Response{}, os.WriteFile(filepath.Join(workspace, ".git", "config"), []byte(config), 0600)
	case "list", "fetch", "prepare", "push", "observe":
	default:
		return Response{}, fmt.Errorf("unsupported trusted Git operation")
	}
	info, err := os.Lstat(filepath.Join(dir, ".git"))
	if err != nil || !info.IsDir() {
		return Response{}, fmt.Errorf("trusted repository is unavailable")
	}
	if req.Operation == "list" || req.Operation == "fetch" {
		return readHeads(git, req, func(input []byte) (int64, error) {
			return runPack(trustedCommand(ctx, dir, "pack-objects", "--stdout", "--revs"), bytes.NewReader(input), req.PackOutput)
		})
	}
	if req.Operation == "observe" {
		if req.OldOID != "" || req.NewOID != "" || req.Pack != nil || req.Workspace != "" {
			return Response{}, fmt.Errorf("invalid remote observation")
		}
		oid, err := observeHead(git, req.Remote, req.Ref)
		return Response{OID: oid, Ref: req.Ref}, err
	}
	return pushOperation(git, req, func(input io.Reader) error {
		_, err := runPack(trustedCommand(ctx, dir, "index-pack", "--stdin", "--strict", fmt.Sprintf("--max-input-size=%d", maxTransferBytes)), input, io.Discard)
		return err
	})
}

func trustedCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	options := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.attributesFile=/dev/null", "-c", "core.pager=cat", "-c", "color.ui=false", "-c", "credential.helper=", "-c", "credential.helper=!/usr/bin/gh auth git-credential", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.file.allow=always", "-c", "fetch.fsckObjects=true", "-c", "transfer.fsckObjects=true", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	if dir != "" {
		options = append(options, "-C", dir)
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/git", append(options, args...)...)
	// Only the trusted Host's gh store is consulted. No caller environment,
	// global Git configuration, replace refs, hooks or external diff is loaded.
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/root", "LANG=C.UTF-8", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1"}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	return cmd
}

func trustedGit(ctx context.Context, dir string, input []byte, args ...string) ([]byte, error) {
	var out cappedBuffer
	out.limit = maxCommandOutput
	_, err := runPack(trustedCommand(ctx, dir, args...), bytes.NewReader(input), &out)
	if err != nil {
		return nil, fmt.Errorf("trusted Git %s failed", args[0])
	}
	return out.Bytes(), nil
}
