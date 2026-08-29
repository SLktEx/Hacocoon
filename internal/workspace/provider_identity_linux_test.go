//go:build linux

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExternalPathWorkspaceIdentitySurvivesRename(t *testing.T) {
	root := t.TempDir()
	beforePath := filepath.Join(root, "before")
	afterPath := filepath.Join(root, "after")
	if err := os.Mkdir(beforePath, 0o755); err != nil {
		t.Fatal(err)
	}
	provider := NewExternalPathWorkspace()
	before, err := provider.Resolve(context.Background(), WorkspaceRequest{Path: beforePath})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(beforePath, afterPath); err != nil {
		t.Fatal(err)
	}
	after, err := provider.Resolve(context.Background(), WorkspaceRequest{Path: afterPath})
	if err != nil {
		t.Fatal(err)
	}
	if before.ID != after.ID {
		t.Fatalf("workspace identity changed across rename: before=%q after=%q", before.ID, after.ID)
	}
	if before.Path == after.Path {
		t.Fatalf("test did not exercise distinct paths: %q", before.Path)
	}
}
