package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExternalPathWorkspaceNormalizesIdentity(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "workspace")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}

	provider := NewExternalPathWorkspace()
	direct, err := provider.Resolve(context.Background(), WorkspaceRequest{Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	throughLink, err := provider.Resolve(context.Background(), WorkspaceRequest{Path: link})
	if err != nil {
		t.Fatal(err)
	}
	if direct.ID == "" || direct.ID != throughLink.ID {
		t.Fatalf("workspace ids direct=%q link=%q", direct.ID, throughLink.ID)
	}
	if direct.Path != dir || throughLink.Path != dir {
		t.Fatalf("paths direct=%q link=%q want=%q", direct.Path, throughLink.Path, dir)
	}
}
