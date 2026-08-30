package gitcap

import (
	"context"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	brokeredEnvPath = "/usr/bin/env"
	brokeredGitPath = "/usr/bin/git"

	// Keep HTTPS GitHub authentication host-owned without trusting the host's
	// complete ~/.gitconfig. The command is constant and Git only invokes it
	// when github.com asks for HTTPS credentials. gh reads the host user's auth
	// state via HOME, which is intentionally forwarded below.
	brokeredGitHubCredentialHelper = "!gh auth git-credential"
)

type isolatedGitRunner struct {
	base host.Runner
}

func newIsolatedGitRunner(base host.Runner) host.Runner {
	if base == nil {
		return nil
	}
	if _, ok := base.(isolatedGitRunner); ok {
		return base
	}
	return isolatedGitRunner{base: base}
}

func (r isolatedGitRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	if name != "git" {
		return r.base.Run(ctx, name, args...)
	}

	// Brokered Git is a host-authority boundary. Do not inherit arbitrary
	// GIT_*, askpass, SSH-command, proxy, or global/system configuration from
	// the Hacocoon process. Only the small set below is intentionally carried
	// into Git. SSH may use the host user's default keys/known_hosts or agent,
	// but ~/.ssh/config is disabled so it cannot rewrite github.com transport.
	env := []string{
		"-i",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=/usr/bin/false",
		"SSH_ASKPASS=/usr/bin/false",
		"GIT_SSH_COMMAND=/usr/bin/ssh -F /dev/null -o BatchMode=yes -o ClearAllForwardings=yes -o PermitLocalCommand=no",
	}
	for _, key := range []string{"HOME", "SSH_AUTH_SOCK", "LANG", "LC_ALL"} {
		if value := os.Getenv(key); value != "" && !strings.ContainsRune(value, '\x00') {
			env = append(env, key+"="+value)
		}
	}

	// Do not re-enable ~/.gitconfig just to obtain credentials. Instead clear
	// every inherited/repository helper in the trusted command line config and
	// install the single host-owned gh provider for github.com HTTPS remotes.
	// This preserves the transport/config isolation while supporting the same
	// account the operator configured with `gh auth setup-git`.
	gitArgs := []string{
		"-c", "credential.helper=",
		"-c", "credential.https://github.com.helper=" + brokeredGitHubCredentialHelper,
	}
	gitArgs = append(gitArgs, args...)

	env = append(env, brokeredGitPath)
	env = append(env, gitArgs...)
	return r.base.Run(ctx, brokeredEnvPath, env...)
}
