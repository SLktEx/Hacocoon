package gitcap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestIsolatedGitRunnerIgnoresGlobalURLRewrite(t *testing.T) {
	requireBrokeredGitTools(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	attacker := filepath.Join(root, "attacker.git")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	setupRepository(t, repo)

	t.Setenv("HOME", home)
	runDirectGit(t, []string{"config", "--global", "url.file://" + attacker + ".insteadOf", "https://github.com/"}, "HOME="+home)
	poisoned := runDirectGit(t, []string{"-C", repo, "remote", "get-url", "origin"}, "HOME="+home)
	if !strings.HasPrefix(strings.TrimSpace(poisoned), "file://"+attacker) {
		t.Fatalf("global rewrite test precondition failed: %q", poisoned)
	}

	result, err := newIsolatedGitRunner(host.ExecRunner{}).Run(context.Background(), "git", "-C", repo, "remote", "get-url", "origin")
	if err != nil {
		t.Fatalf("isolated git: %v stderr=%q", err, result.Stderr)
	}
	if got := strings.TrimSpace(result.Stdout); got != "https://github.com/acme/demo.git" {
		t.Fatalf("global url rewrite escaped isolation: %q", got)
	}
}

func TestIsolatedGitRunnerIgnoresGitConfigCountInjection(t *testing.T) {
	requireBrokeredGitTools(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	attacker := filepath.Join(root, "attacker.git")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	setupRepository(t, repo)

	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url.file://"+attacker+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/")
	poisoned := runDirectGit(t, []string{"-C", repo, "remote", "get-url", "origin"},
		"HOME="+home,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=url.file://"+attacker+".insteadOf",
		"GIT_CONFIG_VALUE_0=https://github.com/",
	)
	if !strings.HasPrefix(strings.TrimSpace(poisoned), "file://"+attacker) {
		t.Fatalf("GIT_CONFIG_COUNT test precondition failed: %q", poisoned)
	}

	result, err := newIsolatedGitRunner(host.ExecRunner{}).Run(context.Background(), "git", "-C", repo, "remote", "get-url", "origin")
	if err != nil {
		t.Fatalf("isolated git: %v stderr=%q", err, result.Stderr)
	}
	if got := strings.TrimSpace(result.Stdout); got != "https://github.com/acme/demo.git" {
		t.Fatalf("GIT_CONFIG_COUNT escaped isolation: %q", got)
	}
}

func setupRepository(t *testing.T, repo string) {
	t.Helper()
	runDirectGit(t, []string{"init", repo})
	runDirectGit(t, []string{"-C", repo, "remote", "add", "origin", "https://github.com/acme/demo.git"})
}

func requireBrokeredGitTools(t *testing.T) {
	t.Helper()
	for _, path := range []string{brokeredEnvPath, brokeredGitPath} {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			t.Skipf("required brokered Git tool is unavailable: %s", path)
		}
	}
}

func runDirectGit(t *testing.T, args []string, extraEnv ...string) string {
	t.Helper()
	cmd := exec.Command(brokeredGitPath, args...)
	cmd.Env = append([]string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"GIT_TERMINAL_PROMPT=0",
	}, extraEnv...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
