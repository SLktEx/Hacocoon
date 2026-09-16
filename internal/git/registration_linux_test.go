//go:build linux

package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Ordinary independent filesystem copies exercise the service and real Git
// agent together without claiming Incus volume acceptance.
type registrationBackend struct{ localBackend }

func (b registrationBackend) CreateVolume(_ context.Context, object Object, source *Object) error {
	if source == nil {
		return os.MkdirAll(filepath.Join(b.repos, object.ID), 0700)
	}
	return os.CopyFS(filepath.Join(b.workspaces, object.ID), os.DirFS(filepath.Join(b.repos, source.ID)))
}

func (b registrationBackend) Populate(ctx context.Context, object Object) error {
	operation := "clone"
	if object.Kind == "work" {
		operation = "workspace"
	}
	_, err := b.RunGit(ctx, gitadapter.AgentRequest{Operation: operation, Repository: object.Repository, Workspace: object.ID, Remote: object.Remote, Branch: object.Branch})
	return err
}

func TestRegistrationIsBranchAgnosticAcrossIndependentWorkspaces(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	remote, seed := filepath.Join(root, "remote.git"), filepath.Join(root, "seed")
	for _, dir := range []string{remote, seed} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=release/v1.2")
	testGit(t, seed, "init", "--initial-branch=release/v1.2")
	initial := testCommit(t, seed, "work.txt", "initial")
	testGit(t, seed, "push", "file://"+remote, "release/v1.2")
	testGit(t, seed, "tag", "tag/HEAD")
	testGit(t, seed, "push", "file://"+remote, "refs/tags/tag/HEAD")
	backend := registrationBackend{localBackend{repos: filepath.Join(root, "repos"), workspaces: filepath.Join(root, "workspaces")}}
	service := NewRepositoryService(filepath.Join(root, "state"), backend)
	source, err := service.Add(ctx, "sample", "file://"+remote)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(service.path("repo", "sample"))
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if _, exists := stored["branch"]; exists || source.Branch != "" {
		t.Fatal("registration persisted checkout selection", string(data))
	}
	if _, err := os.Stat(filepath.Join(backend.repos, "sample", "work.txt")); !os.IsNotExist(err) {
		t.Fatal("registration checked out a branch", err)
	}

	// These branches did not exist when the source was registered.
	testGit(t, seed, "switch", "-c", "main")
	main := testCommit(t, seed, "work.txt", "main")
	testGit(t, seed, "push", "file://"+remote, "main")
	testGit(t, seed, "switch", "-c", "feature/foo")
	feature := testCommit(t, seed, "work.txt", "feature")
	testGit(t, seed, "push", "file://"+remote, "feature/foo")

	for _, tc := range []struct{ id, branch, wantBranch, oid string }{
		{"default", "", "release/v1.2", initial},
		{"main", "main", "main", main},
		{"feature", "feature/foo", "feature/foo", feature},
	} {
		work, err := service.CopyWorkspaceBranch(ctx, tc.id, source.ID, tc.branch)
		if err != nil {
			t.Fatal(err)
		}
		if work.Repository != source.ID || work.Remote != source.Remote || work.Branch != tc.branch {
			t.Fatal(work)
		}
		dir := filepath.Join(backend.workspaces, tc.id)
		if got := testGit(t, dir, "rev-parse", "HEAD"); got != tc.oid {
			t.Fatal(tc.id, got)
		}
		if got := testGit(t, dir, "symbolic-ref", "--short", "HEAD"); got != tc.wantBranch {
			t.Fatal(tc.id, got)
		}
		if got := testGit(t, dir, "config", "remote.origin.url"); got != "haco://sample" {
			t.Fatal("upstream leaked", got)
		}
		if got := testGit(t, dir, "config", "branch."+tc.wantBranch+".merge"); got != "refs/heads/"+tc.wantBranch {
			t.Fatal(got)
		}
	}
	// A changed default is observed at preparation time, without re-registering.
	testGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/feature/foo")
	if _, err := service.CopyWorkspace(ctx, "new-default", source.ID); err != nil {
		t.Fatal(err)
	}
	if got := testGit(t, filepath.Join(backend.workspaces, "new-default"), "rev-parse", "HEAD"); got != feature {
		t.Fatal("registration pinned the default", got)
	}
	if got := testGit(t, filepath.Join(backend.workspaces, "default"), "rev-parse", "HEAD"); got != initial {
		t.Fatal("existing Workspace changed", got)
	}
	testCommit(t, filepath.Join(backend.workspaces, "main"), "work.txt", "independent")
	if got := testGit(t, filepath.Join(backend.workspaces, "feature"), "rev-parse", "HEAD"); got != feature {
		t.Fatal("shared writable Git metadata", got)
	}
	after, err := service.Get("repo", source.ID)
	if err != nil || !reflect.DeepEqual(after, source) {
		t.Fatal("Workspace branch changed source identity", after, err)
	}
	if _, err := service.CopyWorkspaceBranch(ctx, "unsafe", source.ID, "--orphan=bad"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := os.Stat(service.path("work", "unsafe")); !os.IsNotExist(err) {
		t.Fatal("invalid selection allocated data", err)
	}
	if _, err := service.CopyWorkspaceBranch(ctx, "missing", source.ID, "absent"); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("missing branch succeeded or lost ownership", err)
	}
	if _, err := service.Get("work", "missing"); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	// Registration and explicit checkout do not depend on any default branch.
	testGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/nonexistent")
	if _, err := service.Add(ctx, "headless", "file://"+remote); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CopyWorkspaceBranch(ctx, "headless-main", "headless", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CopyWorkspace(ctx, "headless-default", "headless"); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("unavailable default guessed", err)
	}
	listed, err := backend.RunGit(ctx, gitadapter.AgentRequest{Operation: "list", Repository: "headless", Remote: "file://" + remote})
	if err != nil || len(listed.Heads) != 3 || listed.Ref != "" || listed.OID != "" {
		t.Fatal("missing HEAD hid valid branches", listed, err)
	}
	var output bytes.Buffer
	if err := gitadapter.Helper(ctx, []string{"origin", "haco://headless"}, strings.NewReader("list\n\n"), &output, &output, func(context.Context, gitadapter.Request) (gitadapter.Response, error) { return listed, nil }); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), " HEAD\n") || !strings.Contains(output.String(), "refs/heads/main\n") {
		t.Fatal(output.String())
	}
}
