package ec2

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateWorkspaceArchiveKeepsTarTargetInsidePrivateDirectory(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "input.txt"), []byte("safe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	archive, err := createWorkspaceArchive(context.Background(), runner, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(archive))

	info, err := os.Stat(filepath.Dir(archive))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("archive directory is accessible to other users: mode=%#o", info.Mode().Perm())
	}
	if filepath.Base(archive) != "workspace.tgz" {
		t.Fatalf("archive path is not contained under a dedicated directory: %s", archive)
	}
}

func TestRestoreDoesNotDeletePreexistingBackupNamedSibling(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	victim := workspace + ".haco-backup"
	if err := os.Mkdir(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(victim, "must-survive.txt")
	if err := os.WriteFile(marker, []byte("do not delete\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(parent, "remote.tgz")
	if err := os.WriteFile(archive, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreWorkspaceArchive(context.Background(), &fakeRunner{}, archive, workspace); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(marker); err != nil || string(content) != "do not delete\n" {
		t.Fatalf("preexisting sibling backup was modified or deleted: content=%q err=%v", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(workspace, "remote.txt")); err != nil || string(content) != "from-ec2\n" {
		t.Fatalf("restored workspace missing remote content: content=%q err=%v", content, err)
	}
}
