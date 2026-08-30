package ec2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestWorkspaceDigestTracksContentModeAndSymlinkButIgnoresMtime(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "file.txt")
	if err := os.WriteFile(file, []byte("one\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(workspace, "link")
	if err := os.Symlink("file.txt", link); err != nil {
		t.Fatal(err)
	}

	base := mustWorkspaceDigest(t, workspace)
	stamp := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(file, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if got := mustWorkspaceDigest(t, workspace); got != base {
		t.Fatalf("mtime-only change altered workspace identity: base=%s got=%s", base, got)
	}

	if err := os.WriteFile(file, []byte("two\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	contentDigest := mustWorkspaceDigest(t, workspace)
	if contentDigest == base {
		t.Fatal("content change was not detected")
	}

	if err := os.Chmod(file, 0o600); err != nil {
		t.Fatal(err)
	}
	modeDigest := mustWorkspaceDigest(t, workspace)
	if modeDigest == contentDigest {
		t.Fatal("permission change was not detected")
	}

	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("other.txt", link); err != nil {
		t.Fatal(err)
	}
	if got := mustWorkspaceDigest(t, workspace); got == modeDigest {
		t.Fatal("symlink target change was not detected")
	}
}

func TestCreatePersistsStableHostWorkspaceBaseDigest(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	created, err := newTestRuntime(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := decodeRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !validWorkspaceDigest(ref.BaseDigest) {
		t.Fatalf("invalid persisted base digest %q", ref.BaseDigest)
	}
	if want := mustWorkspaceDigest(t, workspace); ref.BaseDigest != want {
		t.Fatalf("base digest=%s want=%s", ref.BaseDigest, want)
	}
}

func TestDeleteRefusesToOverwriteHostWorkspaceChangedWhileEC2WasActive(t *testing.T) {
	workspace := t.TempDir()
	local := filepath.Join(workspace, "local.txt")
	if err := os.WriteFile(local, []byte("creation-base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := mustWorkspaceDigest(t, workspace)
	raw, err := encodeRef(runtimeRef{
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: workspace,
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
		BaseDigest:    base,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("host-edited-after-create\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{}
	err = newTestRuntime(runner).DeleteEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrWorkspaceBusy) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v, want workspace conflict + recovery required", err)
	}
	content, readErr := os.ReadFile(local)
	if readErr != nil || string(content) != "host-edited-after-create\n" {
		t.Fatalf("host edit was overwritten: content=%q err=%v", content, readErr)
	}
	joined := strings.Join(runner.calls, "\n")
	if !strings.Contains(joined, "s3 cp /tmp/haco-output.tgz") {
		t.Fatalf("remote recovery output was not staged before conflict:\n%s", joined)
	}
	for _, forbidden := range []string{"tar -xzf", "s3api put-object", "ec2 terminate-instances", "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("conflict caused destructive step %q:\n%s", forbidden, joined)
		}
	}
}

func TestDeleteLegacyRWRefWithoutBaseDigestFailsClosed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "local.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := runtimeRef{
		Version:       1,
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: workspace,
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
		ReadOnly:      false,
	}
	payload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	raw := "ec2v1." + base64.RawURLEncoding.EncodeToString(payload)

	runner := &fakeRunner{}
	err = newTestRuntime(runner).DeleteEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	joined := strings.Join(runner.calls, "\n")
	if strings.Contains(joined, "tar -xzf") || strings.Contains(joined, "terminate-instances") || strings.Contains(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive") {
		t.Fatalf("legacy ref without base proof mutated host/runtime:\n%s", joined)
	}
	content, readErr := os.ReadFile(filepath.Join(workspace, "local.txt"))
	if readErr != nil || string(content) != "keep\n" {
		t.Fatalf("legacy recovery altered host workspace: content=%q err=%v", content, readErr)
	}
}

func TestCreateFailsBeforeAWSWhenWorkspaceChangesDuringArchive(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "base.txt")
	if err := os.WriteFile(file, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &mutateArchiveRunner{workspaceFile: file}
	_, err := newTestRuntime(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace})
	if !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("err=%v", err)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "aws ") {
			t.Fatalf("AWS was mutated after unstable base archive: %v", runner.calls)
		}
	}
}

func TestDecodeRefRejectsMalformedWorkspaceBaseDigest(t *testing.T) {
	ref := runtimeRef{
		Version:       1,
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: "/srv/hacocoon/workspaces/demo",
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
		ReadOnly:      false,
		BaseDigest:    "sha256:not-a-digest",
	}
	payload, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	raw := "ec2v1." + base64.RawURLEncoding.EncodeToString(payload)
	if _, err := decodeRef(raw); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
}

type mutateArchiveRunner struct {
	calls         []string
	workspaceFile string
	mutated       bool
}

func (r *mutateArchiveRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if name == "tar" && len(args) > 0 && args[0] == "-czf" {
		if !r.mutated {
			r.mutated = true
			if err := os.WriteFile(r.workspaceFile, []byte("changed-during-archive\n"), 0o644); err != nil {
				return host.Result{}, err
			}
		}
		if len(args) >= 2 {
			if err := os.WriteFile(args[1], []byte("archive"), 0o600); err != nil {
				return host.Result{}, err
			}
		}
	}
	return host.Result{}, nil
}

func mustWorkspaceDigest(t *testing.T, workspace string) string {
	t.Helper()
	digest, err := digestWorkspace(context.Background(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
