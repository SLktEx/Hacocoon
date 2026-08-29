package gitcap_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	gitcapapp "github.com/SLktEx/Hacocoon/internal/gitcap"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
)

func TestBrokeredPushWithRealGitAndGitHubShapedRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	bare := filepath.Join(root, "remote.git")
	runGit(t, "init", "--bare", bare)
	runGit(t, "init", workspace)
	runGit(t, "-C", workspace, "config", "user.email", "test@example.invalid")
	runGit(t, "-C", workspace, "config", "user.name", "Hacocoon Test")
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, "-C", workspace, "add", "README.md")
	runGit(t, "-C", workspace, "commit", "-m", "first")
	first := strings.TrimSpace(runGit(t, "-C", workspace, "rev-parse", "HEAD"))
	runGit(t, "-C", workspace, "remote", "add", "origin", "https://github.com/acme/demo.git")
	runGit(t, "-C", workspace, "config", "url.file://"+bare+".insteadOf", "https://github.com/acme/demo.git")

	statePath := filepath.Join(root, "state", "environments.json")
	store := state.NewEnvironmentJSONStore(statePath)
	if err := store.PutEnvironment(ctx, core.Environment{Name: "demo", Workspace: core.Workspace{ID: "path:test", Path: workspace}, AccessMode: core.WorkspaceReadWrite}); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(root, "policy.json")
	policy := `{"default":"deny","rules":[` +
		`{"capability":"github.git","action":"push","attributes":{"organization":"acme","repository":"demo","target_ref":"refs/heads/feature/test"},"decision":"allow"},` +
		`{"capability":"github.git","action":"force-push","attributes":{"organization":"acme","repository":"demo","target_ref":"refs/heads/main"},"decision":"allow"}` +
		`]}`
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := host.ExecRunner{}
	provider := gitcapapp.NewProvider(runner, store)
	caps := capabilityapp.New(capabilityapp.NewFilePolicyEvaluator(policyPath), nil, capabilityapp.NewJSONLAudit(filepath.Join(root, "audit.jsonl")), provider)
	broker := gitcapapp.NewBroker(runner, store, caps)

	if _, err := broker.Push(ctx, gitcapapp.PushSpec{Environment: "demo", Branch: "feature/test"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(runGit(t, "--git-dir="+bare, "rev-parse", "refs/heads/feature/test")); got != first {
		t.Fatalf("feature ref=%s want=%s", got, first)
	}

	runGit(t, "--git-dir="+bare, "update-ref", "refs/heads/main", first)
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, "-C", workspace, "add", "README.md")
	runGit(t, "-C", workspace, "commit", "-m", "second")
	second := strings.TrimSpace(runGit(t, "-C", workspace, "rev-parse", "HEAD"))
	if _, err := broker.Push(ctx, gitcapapp.PushSpec{Environment: "demo", Branch: "main", Force: true}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(runGit(t, "--git-dir="+bare, "rev-parse", "refs/heads/main")); got != second {
		t.Fatalf("main ref=%s want=%s", got, second)
	}

	if _, err := broker.Push(ctx, gitcapapp.PushSpec{Environment: "demo", Branch: "denied"}); err == nil {
		t.Fatal("default-denied branch unexpectedly pushed")
	}
	if cmd := exec.Command("git", "--git-dir="+bare, "show-ref", "--verify", "--quiet", "refs/heads/denied"); cmd.Run() == nil {
		t.Fatal("denied ref exists")
	}
}

func runGit(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
