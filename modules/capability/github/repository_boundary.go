package gitcap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const maxGitMetadataPointerBytes = 4096

type gitRepositoryBoundary struct {
	WorkTree  string
	GitDir    string
	CommonDir string
	Identity  string
}

func resolveRepositoryBoundary(workspace string) (gitRepositoryBoundary, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" || strings.ContainsAny(workspace, "\r\n\x00") {
		return gitRepositoryBoundary{}, core.ErrInvalidArgument
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("resolve workspace path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if info, statErr := os.Stat(absolute); statErr == nil {
		if !info.IsDir() {
			return gitRepositoryBoundary{}, fmt.Errorf("workspace is not a directory: %w", core.ErrInvalidArgument)
		}
		resolved, resolveErr := filepath.EvalSymlinks(absolute)
		if resolveErr != nil {
			return gitRepositoryBoundary{}, fmt.Errorf("resolve workspace symlinks: %w", resolveErr)
		}
		absolute = resolved
	} else if !os.IsNotExist(statErr) {
		return gitRepositoryBoundary{}, fmt.Errorf("inspect workspace: %w", statErr)
	}

	gitEntry := filepath.Join(absolute, ".git")
	entryInfo, err := os.Lstat(gitEntry)
	if os.IsNotExist(err) {
		// Pin Git to the workspace-local .git path even when it is missing. This
		// makes the eventual Git command fail instead of discovering a parent
		// repository outside the admitted Workspace.
		return newRepositoryBoundary(absolute, gitEntry, gitEntry), nil
	}
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("inspect workspace .git entry: %w", err)
	}
	if entryInfo.Mode()&os.ModeSymlink != 0 {
		return gitRepositoryBoundary{}, fmt.Errorf("symlink .git entries are not trusted: %w", core.ErrPolicyDenied)
	}
	if entryInfo.IsDir() {
		if err := rejectObjectAlternates(gitEntry); err != nil {
			return gitRepositoryBoundary{}, err
		}
		return newRepositoryBoundary(absolute, gitEntry, gitEntry), nil
	}
	if !entryInfo.Mode().IsRegular() {
		return gitRepositoryBoundary{}, fmt.Errorf("unsupported .git entry type: %w", core.ErrPolicyDenied)
	}

	pointer, err := readSingleLineMetadata(gitEntry)
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("read worktree .git pointer: %w", err)
	}
	if !strings.HasPrefix(pointer, "gitdir: ") {
		return gitRepositoryBoundary{}, fmt.Errorf("unsupported .git file format: %w", core.ErrPolicyDenied)
	}
	gitDir, err := resolveExistingDirectory(absolute, strings.TrimPrefix(pointer, "gitdir: "))
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("resolve worktree gitdir: %w", err)
	}

	backlink, err := readSingleLineMetadata(filepath.Join(gitDir, "gitdir"))
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("worktree gitdir lacks a trusted backlink: %w", core.ErrPolicyDenied)
	}
	backlinkPath, err := resolveExistingPath(gitDir, backlink)
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("resolve worktree backlink: %w", core.ErrPolicyDenied)
	}
	expectedEntry, err := filepath.EvalSymlinks(gitEntry)
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("resolve workspace .git entry: %w", err)
	}
	if backlinkPath != expectedEntry {
		return gitRepositoryBoundary{}, fmt.Errorf("worktree gitdir does not belong to this Workspace: %w", core.ErrPolicyDenied)
	}

	commonPointer, err := readSingleLineMetadata(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("worktree gitdir lacks commondir metadata: %w", core.ErrPolicyDenied)
	}
	commonDir, err := resolveExistingDirectory(gitDir, commonPointer)
	if err != nil {
		return gitRepositoryBoundary{}, fmt.Errorf("resolve worktree common dir: %w", core.ErrPolicyDenied)
	}
	worktreesDir := filepath.Dir(gitDir)
	if filepath.Base(worktreesDir) != "worktrees" || filepath.Dir(worktreesDir) != commonDir {
		return gitRepositoryBoundary{}, fmt.Errorf("gitdir is not standard linked-worktree metadata: %w", core.ErrPolicyDenied)
	}
	if err := rejectObjectAlternates(commonDir); err != nil {
		return gitRepositoryBoundary{}, err
	}
	return newRepositoryBoundary(absolute, gitDir, commonDir), nil
}

func newRepositoryBoundary(workTree, gitDir, commonDir string) gitRepositoryBoundary {
	material := "hacocoon-git-repository-v1\x00" + workTree + "\x00" + gitDir + "\x00" + commonDir
	digest := sha256.Sum256([]byte(material))
	return gitRepositoryBoundary{
		WorkTree:  workTree,
		GitDir:    gitDir,
		CommonDir: commonDir,
		Identity:  "sha256:" + hex.EncodeToString(digest[:]),
	}
}

func readSingleLineMetadata(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("metadata file is not a regular non-symlink file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, openedInfo) {
		return "", fmt.Errorf("metadata file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxGitMetadataPointerBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxGitMetadataPointerBytes {
		return "", fmt.Errorf("metadata file exceeds %d bytes", maxGitMetadataPointerBytes)
	}
	line := strings.TrimSuffix(string(data), "\n")
	line = strings.TrimSuffix(line, "\r")
	if line == "" || strings.ContainsAny(line, "\r\n\x00") {
		return "", fmt.Errorf("metadata must contain exactly one safe line")
	}
	return line, nil
}

func resolveExistingDirectory(base, raw string) (string, error) {
	path, err := resolveExistingPath(base, raw)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	return path, nil
}

func resolveExistingPath(base, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\x00") {
		return "", core.ErrInvalidArgument
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func rejectObjectAlternates(commonDir string) error {
	path := filepath.Join(commonDir, "objects", "info", "alternates")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Git object alternates: %w", err)
	}
	if strings.TrimSpace(string(data)) != "" {
		return fmt.Errorf("Git object alternates are outside the trusted repository model: %w", core.ErrPolicyDenied)
	}
	return nil
}

type pinnedGitRunner struct {
	base     host.Runner
	boundary gitRepositoryBoundary
}

func pinGitRepository(base host.Runner, boundary gitRepositoryBoundary) host.Runner {
	if base == nil {
		return nil
	}
	return pinnedGitRunner{base: base, boundary: boundary}
}

func (r pinnedGitRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	if name != "git" {
		return r.base.Run(ctx, name, args...)
	}
	pinned := make([]string, 0, len(args)+2)
	pinned = append(pinned, "--git-dir="+r.boundary.GitDir, "--work-tree="+r.boundary.WorkTree)
	pinned = append(pinned, args...)
	return r.base.Run(ctx, name, pinned...)
}
