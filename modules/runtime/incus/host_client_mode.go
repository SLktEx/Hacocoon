package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	trustedHostClientModeEnvKey = "environment.HACO_CLIENT_MODE"
	trustedHostClientModeValue  = "controller"
)

func (r *Runtime) ensureTrustedHostClientMode(ctx context.Context) error {
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
