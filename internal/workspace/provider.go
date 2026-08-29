package workspace

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type WorkspaceRequest struct {
	Path string
}

type WorkspaceProvider interface {
	Resolve(context.Context, WorkspaceRequest) (core.Workspace, error)
}

type ExternalPathWorkspace struct{}

func NewExternalPathWorkspace() ExternalPathWorkspace { return ExternalPathWorkspace{} }

func (ExternalPathWorkspace) Resolve(_ context.Context, req WorkspaceRequest) (core.Workspace, error) {
	path, err := resolveWorkspacePath(req.Path)
	if err != nil {
		return core.Workspace{}, err
	}
	sum := sha256.Sum256([]byte(path))
	return core.Workspace{
		ID:   core.WorkspaceID(fmt.Sprintf("path:%x", sum)),
		Path: path,
	}, nil
}

func resolveWorkspacePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("workspace path is required: %w", core.ErrInvalidArgument)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve workspace %q: %w", path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat workspace %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory: %w", resolved, core.ErrInvalidArgument)
	}
	return filepath.Clean(resolved), nil
}
