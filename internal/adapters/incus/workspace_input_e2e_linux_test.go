//go:build linux

package incus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/internal/workspace/input"
)

// Called only by the explicit native selection fixture, before its registered
// source is cleaned. It adds no Policy grants or installed-controller overrides.
func nativeWorkspaceInput(t *testing.T, ctx context.Context, svc *workspace.Service, repos *gitrepo.RepositoryService, dir, name, repository string, run func(...string) string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	main := filepath.Join(dir, "client-source")
	must(os.Mkdir(main, 0700))
	git := func(path string, args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "/usr/bin/git", append([]string{"-C", path, "-c", "core.hooksPath=/dev/null", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("local Git fixture: %v: %s", err, out)
		}
		return string(out)
	}
	git(main, "init", "--initial-branch=main")
	must(os.WriteFile(filepath.Join(main, "tracked"), []byte("initial"), 0600))
	git(main, "add", "tracked")
	git(main, "commit", "-m", "initial")
	linked := filepath.Join(dir, "client-linked")
	git(main, "worktree", "add", "-b", "feature/input", linked)
	must(os.WriteFile(filepath.Join(linked, "tracked"), []byte("staged"), 0600))
	git(linked, "add", "tracked")
	must(os.WriteFile(filepath.Join(linked, "tracked"), []byte("dirty"), 0600))
	must(os.WriteFile(filepath.Join(linked, "extra"), []byte("untracked"), 0600))
	archive, err := workspaceinput.Capture(ctx, linked, repository)
	must(err)
	defer func() { _ = archive.Close() }()
	work, err := repos.ImportWorkspaceTree(ctx, name+"-input", repository, archive)
	must(err)
	env, err := svc.Create(ctx, core.EnvironmentSpec{Name: name + "-input", WorkspacePath: "managed:" + work.ID, ExpectedWorkspace: core.WorkspaceID("workspace:managed:" + work.Owner), Base: "fixture-parent", SkipDefaultResource: true})
	must(err)
	live := true
	defer func() {
		if live {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
			defer cancel()
			if err := svc.Delete(cleanup, env.Name); err != nil {
				t.Error(err)
			}
		}
	}()
	got := run("exec", env.RuntimeRef, "--project", "hacocoon", "--", "sh", "-ceu", `cd /workspace; cat tracked extra .git/HEAD; test ! -e .git/commondir; test ! -e .git/gitdir; test ! -e .git/worktrees`)
	if got != "dirtyuntrackedref: refs/heads/feature/input\n" {
		t.Fatal("worktree input lost Git/files", got)
	}
	run("exec", env.RuntimeRef, "--project", "hacocoon", "--", "sh", "-ceu", `printf independent > /workspace/tracked`)
	original, err := os.ReadFile(filepath.Join(linked, "tracked"))
	must(err)
	if string(original) != "dirty" {
		t.Fatal("guest changed linked source")
	}
	must(svc.Stop(ctx, env.Name))
	must(svc.Start(ctx, env.Name))
	if got := run("exec", env.RuntimeRef, "--project", "hacocoon", "--", "cat", "/workspace/tracked"); !strings.HasPrefix(got, "independent") {
		t.Fatal("imported data lost on restart")
	}
	must(svc.Delete(ctx, env.Name))
	live = false
	must(svc.CleanupRestoredData(ctx, env.Workspace, func(ctx context.Context) error { return repos.DeleteWorkspace(ctx, work.ID, work.Owner) }))
	t.Log("PASS actual linked-worktree capture/import into an independent managed volume, selected Git state/files, normal Env create/restart, source isolation and exact cleanup; no authenticated push, installed CLI or performance claim")
}
