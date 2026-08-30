package gitcap_test

import (
	"context"
	"errors"
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

func TestBrokeredPushRejectsRepositoryURLRewriteWithRealGit(t *testing.T) {
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
	runGit(t, "-C", workspace, "remote", "add", "origin", "https://github.com/acme/demo.git")

	// This used to be the integration test's trick for making a GitHub-shaped
	// remote write to a local repository. That same trick is an authorization
	// bypass when the workspace is attacker-controlled, so it must now fail.
	runGit(t, "-C", workspace, "config", "url.file://"+bare+".insteadOf", "https://github.com/acme/demo.git")

	store := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
	if err := store.PutEnvironment(ctx, core.Environment{Name: "demo", Workspace: core.Workspace{ID: "path:test", Path: workspace}, AccessMode: core.WorkspaceReadWrite}); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(root, "policy.json")
	policy := `{"default":"deny","rules":[` +
		`{"capability":"github.git","action":"push","resource":"github://acme/demo/refs/heads/feature/test","environment":"demo","attributes":{"organization":"acme","repository":"demo","remote":"origin","source_sha":"*","target_ref":"refs/heads/feature/test"},"decision":"allow"}` +
		`]}`
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := host.ExecRunner{}
	provider := gitcapapp.NewProvider(runner, store)
	caps, err := capabilityapp.New(capabilityapp.NewFilePolicyEvaluator(policyPath), nil, capabilityapp.NewJSONLAudit(filepath.Join(root, "audit", "capabilities.jsonl")), provider)
	if err != nil {
		t.Fatal(err)
	}
	broker := gitcapapp.NewBroker(runner, store, caps)

	if _, err := broker.Push(ctx, gitcapapp.PushSpec{Environment: "demo", Branch: "feature/test"}); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("repository-local URL rewrite was not denied: %v", err)
	}
	if cmd := exec.Command("git", "--git-dir="+bare, "show-ref", "--verify", "--quiet", "refs/heads/feature/test"); cmd.Run() == nil {
		t.Fatal("rewritten destination was modified despite policy denial")
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

func containsLine(output, want string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}
