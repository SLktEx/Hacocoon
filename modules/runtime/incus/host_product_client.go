package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const trustedHostProductClientPath = "/usr/local/bin/haco"

// ProvisionTrustedHostProductClient installs the product-facing haco binary in
// the trusted Host. It deliberately does not grant authority or initialize any
// local runtime state; the product CLI decides which controller-backed product
// operations it supports.
func (r *Runtime) ProvisionTrustedHostProductClient(ctx context.Context, source string) error {
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
		return fmt.Errorf("trusted host must be running before product client provisioning, got %q: %w", state, core.ErrIncompatibleState)
	}

	source, digest, err := trustedClientSource(source)
	if err != nil {
		return err
	}
	if ok, _ := r.trustedHostProductClientMatches(ctx, digest); ok {
		return nil
	}

	if _, err := r.runner.Run(ctx, "incus", "file", "push", source,
		trustedHostName+trustedHostProductClientPath,
		"--project", r.project,
		"--create-dirs",
		"--uid", "0",
		"--gid", "0",
		"--mode", "0755",
	); err != nil {
		return fmt.Errorf("install trusted host product client: %w", err)
	}
	ok, verifyErr := r.trustedHostProductClientMatches(ctx, digest)
	if verifyErr != nil {
		return fmt.Errorf("verify trusted host product client: %w", verifyErr)
	}
	if !ok {
		return fmt.Errorf("trusted host product client verification mismatch: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) trustedHostProductClientMatches(ctx context.Context, digest string) (bool, error) {
	hashResult, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "sha256sum", trustedHostProductClientPath)
	if err != nil {
		return false, err
	}
	fields := strings.Fields(hashResult.Stdout)
	if len(fields) < 1 || fields[0] != digest {
		return false, nil
	}
	statResult, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "stat", "-c", "%a:%u:%g", trustedHostProductClientPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(statResult.Stdout) == "755:0:0", nil
}
