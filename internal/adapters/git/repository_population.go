package gitadapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Only controller-owned Repository material enters this operation. Workspace
// directories are handled by the separate workspace operation and never rebuilt.
func populateRepository(ctx context.Context, req AgentRequest, repos, workspaces, dir string) (Response, error) {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		_, err = trustedGit(ctx, "", nil, "clone", "--progress", "--template=", "--no-local", "--no-tags", "--", req.Remote, dir)
		return Response{}, err
	}
	if err != nil || !info.IsDir() {
		return Response{}, fmt.Errorf("invalid owned Repository directory")
	}
	gitDir := filepath.Join(dir, ".git")
	info, err = os.Lstat(gitDir)
	if err == nil {
		if !info.IsDir() {
			return Response{}, fmt.Errorf("invalid Repository Git directory")
		}
		if _, err = trustedGit(ctx, dir, nil, "fsck", "--connectivity-only", "--no-dangling"); err != nil {
			if ctx.Err() != nil {
				return Response{}, ctx.Err()
			}
			quarantine, err := os.MkdirTemp(dir, ".haco-repository-recovery-")
			if err != nil {
				return Response{}, err
			}
			if err = os.Rename(gitDir, filepath.Join(quarantine, "git")); err != nil {
				return Response{}, err
			}
		}
	} else if !os.IsNotExist(err) {
		return Response{}, err
	}
	if _, err := trustedGit(ctx, dir, nil, "init", "--template=", "--"); err != nil {
		return Response{}, err
	}
	remotes, err := trustedGit(ctx, dir, nil, "remote")
	if err != nil {
		return Response{}, err
	}
	listed := strings.Fields(string(remotes))
	switch {
	case len(listed) == 0:
		if _, err := trustedGit(ctx, dir, nil, "remote", "add", "origin", req.Remote); err != nil {
			return Response{}, err
		}
	case len(listed) == 1 && listed[0] == "origin":
		origin, err := trustedGit(ctx, dir, nil, "remote", "get-url", "origin")
		if err != nil || strings.TrimSpace(string(origin)) != req.Remote {
			return Response{}, fmt.Errorf("trusted repository remote changed")
		}
	default:
		return Response{}, fmt.Errorf("trusted repository has unexpected remotes")
	}
	resolve := req
	resolve.Operation = "resolve"
	result, err := RunAgent(ctx, resolve, repos, workspaces)
	if err != nil {
		return Response{}, err
	}
	if _, err := trustedGit(ctx, dir, nil, "checkout", "--force", "-B", result.Ref[len("refs/heads/"):], "refs/remotes/origin/"+result.Ref[len("refs/heads/"):], "--"); err != nil {
		return Response{}, err
	}
	return Response{}, nil
}
