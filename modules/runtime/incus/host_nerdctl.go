package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const trustedHostNerdctlPath = "/usr/local/bin/nerdctl"

// EnsureTrustedHostNerdctlShim exposes the already-verified haco-host client
// under the conventional nerdctl command name. It never replaces an existing
// unrelated file or symlink and it does not install containerd in haco-host.
func (r *Runtime) EnsureTrustedHostNerdctlShim(ctx context.Context) error {
	state, exists, err := r.trustedHostState(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("trusted host is missing: %w", core.ErrNotFound)
	}
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	if state != "RUNNING" {
		return fmt.Errorf("trusted host must be running before nerdctl shim provisioning, got %q: %w", state, core.ErrIncompatibleState)
	}

	readlink := func() (string, error) {
		result, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
			"--", "readlink", trustedHostNerdctlPath)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(result.Stdout), nil
	}
	if target, readErr := readlink(); readErr == nil {
		if target == trustedHostClientPath {
			return nil
		}
		return fmt.Errorf("trusted host nerdctl path points to %q instead of %q: %w", target, trustedHostClientPath, core.ErrIncompatibleState)
	}

	// Prove the path is absent before creating it. This prevents an installation
	// race or a pre-existing regular file from being silently overwritten.
	if _, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "test", "!", "-e", trustedHostNerdctlPath); err != nil {
		return fmt.Errorf("trusted host nerdctl path exists but is not the managed shim: %w", core.ErrIncompatibleState)
	}
	if _, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "ln", "-s", trustedHostClientPath, trustedHostNerdctlPath); err != nil {
		return fmt.Errorf("install trusted host nerdctl shim: %w", err)
	}
	target, err := readlink()
	if err != nil {
		return fmt.Errorf("verify trusted host nerdctl shim: %w", err)
	}
	if target != trustedHostClientPath {
		return fmt.Errorf("trusted host nerdctl shim verification mismatch: got %q want %q: %w", target, trustedHostClientPath, core.ErrIncompatibleState)
	}
	return nil
}
