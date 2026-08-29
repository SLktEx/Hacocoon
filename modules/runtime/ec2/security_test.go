package ec2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type archiveProbeRunner struct {
	sawTar bool
}

func (r *archiveProbeRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "tar" || len(args) < 2 || args[0] != "-czf" {
		return host.Result{}, fmt.Errorf("unexpected command: %s %v", name, args)
	}
	info, err := os.Lstat(args[1])
	if err != nil {
		return host.Result{}, fmt.Errorf("archive temp path was not pinned before tar: %w", err)
	}
	if !info.Mode().IsRegular() {
		return host.Result{}, fmt.Errorf("archive temp path is not a regular file: %s", info.Mode())
	}
	r.sawTar = true
	if err := os.WriteFile(args[1], []byte("archive"), 0o600); err != nil {
		return host.Result{}, err
	}
	return host.Result{}, nil
}

func TestCreateWorkspaceArchiveKeepsTempPathPinnedUntilTar(t *testing.T) {
	runner := &archiveProbeRunner{}
	archive, err := createWorkspaceArchive(context.Background(), runner, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive)
	if !runner.sawTar {
		t.Fatal("tar was not invoked")
	}
	if info, err := os.Lstat(archive); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("archive=%v err=%v", info, err)
	}
}

func TestSyncBackDownloadsOutsideWorkspaceParent(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	runtime := newTestRuntime(runner)
	ref := runtimeRef{
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: workspace,
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
	}
	if err := runtime.syncBack(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	var destination string
	for _, call := range runner.calls {
		fields := strings.Fields(call)
		for i, field := range fields {
			if strings.HasPrefix(field, "s3://") && strings.HasSuffix(field, "/output.tgz") && i+1 < len(fields) {
				destination = fields[i+1]
				break
			}
		}
		if destination != "" {
			break
		}
	}
	if destination == "" {
		t.Fatalf("remote archive download was not observed: %v", runner.calls)
	}
	if filepath.Clean(filepath.Dir(destination)) == filepath.Clean(parent) {
		t.Fatalf("remote archive was downloaded into attacker-controlled workspace parent: %s", destination)
	}
}

func TestRestoreWorkspaceArchivePreservesExistingRecoveryBackup(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "original.txt"), []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := workspace + ".haco-backup"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(backup, "must-survive.txt")
	if err := os.WriteFile(marker, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(parent, "remote.tgz")
	if err := os.WriteFile(archive, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := restoreWorkspaceArchive(context.Background(), &fakeRunner{}, archive, workspace)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep\n" {
		t.Fatalf("existing recovery backup was damaged: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(workspace, "original.txt")); err != nil || string(got) != "original\n" {
		t.Fatalf("original workspace changed despite backup collision: content=%q err=%v", got, err)
	}
}
