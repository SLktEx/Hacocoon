package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	trustedHostGeneralClientPath = "/usr/local/bin/hacoq"
	trustedHostClientModeEnvKey  = "environment.HACO_CLIENT_MODE"
	trustedHostClientModeValue   = "controller"
)

// ProvisionTrustedHostGeneralClient installs the temporary legacy controller
// client as hacoq only after the trusted Host already exists and is running.
// The explicit client-mode marker makes still-unmigrated legacy commands fail
// closed inside haco-host instead of accidentally initializing guest-local
// Hacocoon state.
func (r *Runtime) ProvisionTrustedHostGeneralClient(ctx context.Context, source string) error {
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
		return fmt.Errorf("trusted host must be running before general client provisioning, got %q: %w", state, core.ErrIncompatibleState)
	}
	if err := r.ensureTrustedHostGeneralClientMode(ctx); err != nil {
		return err
	}

	source, digest, err := trustedClientSource(source)
	if err != nil {
		return err
	}
	if ok, _ := r.trustedHostGeneralClientMatches(ctx, digest); ok {
		return nil
	}

	if _, err := r.runner.Run(ctx, "incus", "file", "push", source,
		trustedHostName+trustedHostGeneralClientPath,
		"--project", r.project,
		"--create-dirs",
		"--uid", "0",
		"--gid", "0",
		"--mode", "0755",
	); err != nil {
		return fmt.Errorf("install trusted host legacy client: %w", err)
	}
	ok, verifyErr := r.trustedHostGeneralClientMatches(ctx, digest)
	if verifyErr != nil {
		return fmt.Errorf("verify trusted host legacy client: %w", verifyErr)
	}
	if !ok {
		return fmt.Errorf("trusted host legacy client verification mismatch: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) ensureTrustedHostGeneralClientMode(ctx context.Context) error {
	result, err := r.runner.Run(ctx, "incus", "config", "get", trustedHostName, trustedHostClientModeEnvKey, "--project", r.project)
	if err != nil {
		return fmt.Errorf("read trusted host client mode: %w", err)
	}
	current := strings.TrimSpace(result.Stdout)
	if current == trustedHostClientModeValue {
		return nil
	}
	if current != "" {
		return fmt.Errorf("trusted host client mode mismatch: got %q want %q: %w", current, trustedHostClientModeValue, core.ErrIncompatibleState)
	}
	if _, err := r.runner.Run(ctx, "incus", "config", "set", trustedHostName,
		trustedHostClientModeEnvKey+"="+trustedHostClientModeValue, "--project", r.project); err != nil {
		return fmt.Errorf("set trusted host client mode: %w", err)
	}
	result, err = r.runner.Run(ctx, "incus", "config", "get", trustedHostName, trustedHostClientModeEnvKey, "--project", r.project)
	if err != nil {
		return fmt.Errorf("verify trusted host client mode: %w", err)
	}
	if strings.TrimSpace(result.Stdout) != trustedHostClientModeValue {
		return fmt.Errorf("trusted host client mode did not converge: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) trustedHostGeneralClientMatches(ctx context.Context, digest string) (bool, error) {
	hashResult, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "sha256sum", trustedHostGeneralClientPath)
	if err != nil {
		return false, err
	}
	fields := strings.Fields(hashResult.Stdout)
	if len(fields) < 1 || fields[0] != digest {
		return false, nil
	}
	statResult, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "stat", "-c", "%a:%u:%g", trustedHostGeneralClientPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(statResult.Stdout) == "755:0:0", nil
}
