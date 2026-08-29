package ec2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type syncBackDownloadProbe struct {
	base            fakeRunner
	workspaceParent string
	checked         bool
}

func (p *syncBackDownloadProbe) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	if name == "aws" {
		for i, arg := range args {
			if strings.HasPrefix(arg, "s3://") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "s3://") {
				target := args[i+1]
				parent := filepath.Dir(target)
				info, err := os.Stat(parent)
				if err != nil {
					return host.Result{}, fmt.Errorf("stat download parent: %w", err)
				}
				if info.Mode().Perm()&0o077 != 0 {
					return host.Result{}, fmt.Errorf("download parent permissions are not private: %o", info.Mode().Perm())
				}
				if filepath.Clean(parent) == filepath.Clean(p.workspaceParent) {
					return host.Result{}, fmt.Errorf("remote archive is downloaded directly into attacker-writable workspace parent")
				}
				p.checked = true
				break
			}
		}
	}
	return p.base.Run(ctx, name, args...)
}

func TestSyncBackDownloadsIntoPrivateDirectory(t *testing.T) {
	workspaceParent := t.TempDir()
	workspace := filepath.Join(workspaceParent, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	probe := &syncBackDownloadProbe{workspaceParent: workspaceParent}
	runtime := newTestRuntime(probe)
	ref := runtimeRef{
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: workspace,
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
	}
	if err := runtime.syncBack(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	if !probe.checked {
		t.Fatal("S3 download target was not checked")
	}
}
