package gitcap

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return host.Result{}, nil
}

func TestIsolatedGitRunnerInjectsOnlyTrustedGitHubCredentialHelper(t *testing.T) {
	base := &recordingRunner{}
	runner := newIsolatedGitRunner(base)
	if _, err := runner.Run(context.Background(), "git", "status", "--short"); err != nil {
		t.Fatal(err)
	}
	if base.name != brokeredEnvPath {
		t.Fatalf("name=%q", base.name)
	}

	wantTail := []string{
		brokeredGitPath,
		"-c", "credential.helper=",
		"-c", "credential.https://github.com.helper=" + brokeredGitHubCredentialHelper,
		"status", "--short",
	}
	if len(base.args) < len(wantTail) {
		t.Fatalf("args too short: %#v", base.args)
	}
	gotTail := base.args[len(base.args)-len(wantTail):]
	if !reflect.DeepEqual(gotTail, wantTail) {
		t.Fatalf("tail=%#v want=%#v", gotTail, wantTail)
	}

	for _, arg := range base.args {
		if arg == "GIT_CONFIG_GLOBAL=" || arg == "GIT_CONFIG_GLOBAL=$HOME/.gitconfig" {
			t.Fatalf("global git config unexpectedly enabled: %#v", base.args)
		}
	}
}

func TestIsolatedGitRunnerMapsOnlyExplicitHostGitHubToken(t *testing.T) {
	t.Setenv(brokeredGitHubTokenEnv, "host-token")
	t.Setenv("GH_TOKEN", "ambient-gh-token")
	t.Setenv("GITHUB_TOKEN", "ambient-github-token")

	base := &recordingRunner{}
	runner := newIsolatedGitRunner(base)
	if _, err := runner.Run(context.Background(), "git", "status", "--short"); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(base.args, "\n")
	if !strings.Contains(joined, "GH_TOKEN=host-token") {
		t.Fatalf("explicit host token was not mapped into isolated Git env: %#v", base.args)
	}
	for _, forbidden := range []string{
		brokeredGitHubTokenEnv + "=host-token",
		"GH_TOKEN=ambient-gh-token",
		"GITHUB_TOKEN=ambient-github-token",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("untrusted ambient credential leaked into isolated Git env: %q in %#v", forbidden, base.args)
		}
	}
}
