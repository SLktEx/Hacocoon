package gitcap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestRepositoryBoundaryPinsNormalRepositoryToWorkspaceGitDir(t *testing.T) {
	workspace := t.TempDir()
	gitDir := filepath.Join(workspace, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects", "info"), 0o755); err != nil {
		t.Fatal(err)
	}

	boundary, err := resolveRepositoryBoundary(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if boundary.WorkTree != workspace || boundary.GitDir != gitDir || boundary.CommonDir != gitDir {
		t.Fatalf("unexpected boundary: %#v", boundary)
	}
	if !strings.HasPrefix(boundary.Identity, "sha256:") || len(boundary.Identity) != len("sha256:")+64 {
		t.Fatalf("unexpected repository identity: %q", boundary.Identity)
	}
}

func TestRepositoryBoundaryAcceptsStandardLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "worktree")
	commonDir := filepath.Join(root, "main", ".git")
	gitDir := filepath.Join(commonDir, "worktrees", "agent-1")
	for _, dir := range []string{workspace, gitDir, filepath.Join(commonDir, "objects", "info")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "gitdir"), []byte(filepath.Join(workspace, ".git")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	boundary, err := resolveRepositoryBoundary(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if boundary.WorkTree != workspace || boundary.GitDir != gitDir || boundary.CommonDir != commonDir {
		t.Fatalf("unexpected linked-worktree boundary: %#v", boundary)
	}
}

func TestRepositoryBoundaryRejectsArbitraryExternalGitdir(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	external := filepath.Join(root, "other", ".git")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".git"), []byte("gitdir: "+external+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveRepositoryBoundary(workspace)
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("arbitrary external gitdir was not rejected: %v", err)
	}
}

func TestRepositoryBoundaryRejectsMismatchedWorktreeBacklink(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	otherWorkspace := filepath.Join(root, "other-workspace")
	commonDir := filepath.Join(root, "main", ".git")
	gitDir := filepath.Join(commonDir, "worktrees", "other")
	for _, dir := range []string{workspace, otherWorkspace, gitDir, filepath.Join(commonDir, "objects", "info")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherWorkspace, ".git"), []byte("placeholder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "gitdir"), []byte(filepath.Join(otherWorkspace, ".git")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveRepositoryBoundary(workspace)
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("mismatched worktree backlink was not rejected: %v", err)
	}
}

func TestRepositoryBoundaryRejectsObjectAlternates(t *testing.T) {
	workspace := t.TempDir()
	alternates := filepath.Join(workspace, ".git", "objects", "info", "alternates")
	if err := os.MkdirAll(filepath.Dir(alternates), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alternates, []byte("/host/other/.git/objects\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveRepositoryBoundary(workspace)
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("object alternates escaped repository boundary: %v", err)
	}
}

func TestPinnedGitRunnerSuppliesExplicitGitDirAndWorkTree(t *testing.T) {
	base := &runnerFunc{fn: func(_ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--git-dir=/work/demo/.git") || !strings.Contains(joined, "--work-tree=/work/demo") {
			t.Fatalf("Git invocation was not pinned: %s", joined)
		}
		return host.Result{}, nil
	}}
	boundary := newRepositoryBoundary("/work/demo", "/work/demo/.git", "/work/demo/.git")
	if _, err := pinGitRepository(base, boundary).Run(context.Background(), "git", "-C", "/work/demo", "status"); err != nil {
		t.Fatal(err)
	}
}
