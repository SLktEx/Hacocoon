package ec2

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type archiveRaceProbeRunner struct {
	checked bool
}

func (r *archiveRaceProbeRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "tar" || len(args) < 2 || args[0] != "-czf" {
		return host.Result{}, fmt.Errorf("unexpected command: %s %v", name, args)
	}
	info, err := os.Lstat(args[1])
	if err != nil {
		return host.Result{}, fmt.Errorf("archive pathname disappeared before tar: %w", err)
	}
	if !info.Mode().IsRegular() {
		return host.Result{}, fmt.Errorf("archive pathname is not a regular file: %s", info.Mode())
	}
	r.checked = true
	if err := os.WriteFile(args[1], []byte("archive"), 0o600); err != nil {
		return host.Result{}, err
	}
	return host.Result{}, nil
}

func TestCreateWorkspaceArchiveKeepsReservedTempPath(t *testing.T) {
	runner := &archiveRaceProbeRunner{}
	archive, err := createWorkspaceArchive(context.Background(), runner, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive)
	if !runner.checked {
		t.Fatal("tar runner never verified the reserved archive path")
	}
	info, err := os.Lstat(archive)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("archive is not regular after creation: %s", info.Mode())
	}
}
