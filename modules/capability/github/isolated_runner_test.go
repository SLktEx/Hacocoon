package gitcap

import (
	"context"
	"reflect"
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
