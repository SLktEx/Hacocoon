package ec2

import (
	"context"
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

	base := mustWorkspaceDigestV2(t, workspace)
	stamp := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(file, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if got := mustWorkspaceDigestV2(t, workspace); got != base {
		t.Fatalf("mtime-only change altered workspace identity: base=%s got=%s", base, got)
	}

	if err := os.WriteFile(file, []byte("two\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	contentDigest := mustWorkspaceDigestV2(t, workspace)
	if contentDigest == base {
		t.Fatal("content change was not detected")
	}
	if err := os.Chmod(file, 0o600); err != nil {
		t.Fatal(err)
	}
	modeDigest := mustWorkspaceDigestV2(t, workspace)
	if modeDigest == contentDigest {
		t.Fatal("permission change was not detected")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("other.txt", link); err != nil {
		t.Fatal(err)
	}
	if got := mustWorkspaceDigestV2(t, workspace); got == modeDigest {
		t.Fatal("symlink target change was not detected")
	}
}

func TestCreatePersistsStableHostWorkspaceDigest(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := newTestRuntime(&fakeRunner{}).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := decodeRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !validWorkspaceDigest(ref.BaseDigest) {
		t.Fatalf("invalid persisted digest %q", ref.BaseDigest)
	}
	if want := mustWorkspaceDigestV2(t, workspace); ref.BaseDigest != want {
		t.Fatalf("base digest=%s want=%s", ref.BaseDigest, want)
	}
}

func TestCreateFailsBeforeAWSMutationWhenWorkspaceChangesDuringArchive(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "base.txt")
	if err := os.WriteFile(file, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &workspaceMutatingRunner{path: file, mutateOn: "tar -czf", content: []byte("changed-during-archive\n")}
	_, err := newTestRuntime(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace})
	if !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("err=%v, want workspace busy", err)
	}
	joined := strings.Join(runner.calls, "\n")
	if strings.Contains(joined, " s3 cp ") || strings.Contains(joined, " ec2 run-instances ") || strings.Contains(joined, " ssm send-command ") {
		t.Fatalf("unstable archive reached AWS mutation:\n%s", joined)
	}
}

func TestDeleteRefusesHostEditBeforeRemoteSyncAndRetainsEvidence(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "local.txt")
	if err := os.WriteFile(file, []byte("creation-base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := testRuntimeRef("i-0123456789abcdef0", workspace, "hacocoon-workspaces-example", "tests/demo", false)
	raw, err := encodeRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("host-edited-after-create\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	err = newTestRuntime(runner).DeleteEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrWorkspaceBusy) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v, want workspace conflict + recovery required", err)
	}
	content, readErr := os.ReadFile(file)
	if readErr != nil || string(content) != "host-edited-after-create\n" {
		t.Fatalf("host edit was altered: content=%q err=%v", content, readErr)
	}
	joined := strings.Join(runner.calls, "\n")
	for _, forbidden := range []string{"ssm send-command", "tar -xzf", "ec2 terminate-instances", "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("pre-sync conflict caused destructive step %q:\n%s", forbidden, joined)
		}
	}
}

func TestDeleteCatchesHostEditDuringRemoteDownloadBeforeRestore(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "local.txt")
	if err := os.WriteFile(file, []byte("creation-base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := encodeRef(testRuntimeRef("i-0123456789abcdef0", workspace, "hacocoon-workspaces-example", "tests/demo", false))
	if err != nil {
		t.Fatal(err)
	}
	runner := &workspaceMutatingRunner{
		path:     file,
		mutateOn: "s3 cp s3://hacocoon-workspaces-example/tests/demo/output.tgz",
		content:  []byte("edited-during-download\n"),
	}
	err = newTestRuntime(runner).DeleteEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrWorkspaceBusy) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v, want workspace conflict + recovery required", err)
	}
	content, readErr := os.ReadFile(file)
	if readErr != nil || string(content) != "edited-during-download\n" {
		t.Fatalf("host edit was overwritten: content=%q err=%v", content, readErr)
	}
	joined := strings.Join(runner.calls, "\n")
	if !strings.Contains(joined, "s3 cp /tmp/haco-output.tgz") || !strings.Contains(joined, "s3 cp s3://hacocoon-workspaces-example/tests/demo/output.tgz") {
		t.Fatalf("test did not reach remote stage/download:\n%s", joined)
	}
	for _, forbidden := range []string{"tar -xzf", "ec2 terminate-instances", "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("mid-sync conflict caused destructive step %q:\n%s", forbidden, joined)
		}
	}
}

func TestDeleteRunningRWRefWithoutBaseDigestFailsClosed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "keep.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := runtimeRef{
		AccountID:     testAWSAccountID,
		Region:        "ap-northeast-1",
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: workspace,
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
		ReadOnly:      false,
	}
	raw, err := encodeRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	err = newTestRuntime(runner).DeleteEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	joined := strings.Join(runner.calls, "\n")
	if strings.Contains(joined, "ssm send-command") || strings.Contains(joined, "terminate-instances") || strings.Contains(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive") {
		t.Fatalf("missing base proof reached destructive lifecycle:\n%s", joined)
	}
}

func TestDecodeRefRejectsMalformedWorkspaceDigest(t *testing.T) {
	ref := testRuntimeRef("i-0123456789abcdef0", t.TempDir(), "hacocoon-workspaces-example", "tests/demo", false)
	ref.BaseDigest = "sha256:not-a-digest"
	raw, err := encodeRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRef(raw); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v, want incompatible state", err)
	}
}

type workspaceMutatingRunner struct {
	fakeRunner
	path     string
	mutateOn string
	content  []byte
	mutated  bool
}

func (r *workspaceMutatingRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	call := name + " " + strings.Join(args, " ")
	if !r.mutated && strings.Contains(call, r.mutateOn) {
		r.mutated = true
		if err := os.WriteFile(r.path, r.content, 0o644); err != nil {
			return host.Result{}, err
		}
	}
	return r.fakeRunner.Run(ctx, name, args...)
}

func mustWorkspaceDigestV2(t *testing.T, workspace string) string {
	t.Helper()
	digest, err := digestWorkspace(context.Background(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
