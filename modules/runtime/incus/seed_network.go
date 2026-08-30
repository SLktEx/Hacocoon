package incus

import (
	"context"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// createToolingBuilderNetwork creates a short-lived managed bridge for the
// trusted tooling Base preparation phase. It is intentionally separate from
// the normal sandbox network: ordinary Environments stay behind the
// default-deny proxy-only transport guard, while an explicitly requested Seed
// build may fetch public OS tooling before the published tooling Base is made
// networkless.
func (p *SandboxProvider) createToolingBuilderNetwork(ctx context.Context) (string, func(error) error, error) {
	if p == nil || p.Runtime == nil || p.runner == nil {
		return "", nil, core.ErrInvalidArgument
	}

	name, err := newSeedBuilderName("tooling-net")
	if err != nil {
		return "", nil, err
	}
	result, err := p.runner.Run(ctx, "incus", "network", "create", name,
		"ipv4.address=auto",
		"ipv4.nat=true",
		"ipv4.firewall=true",
		"ipv4.routing=true",
		"ipv6.address=none",
		"--project", sandboxResourceProject,
	)
	if err != nil || result.ExitCode != 0 {
		return "", nil, fmt.Errorf("create trusted tooling builder network: %w", commandResultError(result, err))
	}

	cleanup := func(cause error) error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), p.cleanupTimeout)
		defer cancel()
		result, err := p.runner.Run(cleanupCtx, "incus", "network", "delete", name, "--project", sandboxResourceProject)
		if err == nil && result.ExitCode == 0 {
			return cause
		}
		cleanupErr := fmt.Errorf("cleanup tooling builder network %s: %w", name, commandResultError(result, err))
		if cause == nil {
			return errors.Join(cleanupErr, core.ErrRecoveryRequired)
		}
		return errors.Join(cause, cleanupErr, core.ErrRecoveryRequired)
	}
	return name, cleanup, nil
}
