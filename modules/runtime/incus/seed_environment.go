package incus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const seedEnvironmentRuntimeRetryInterval = 100 * time.Millisecond

// ensureSeedEnvironmentRuntimeReady makes a Seed-derived Environment usable
// before CreateEnvironment returns. Ordinary Base-only Environments never take
// this path. Seed publication intentionally quiesces containerd and the Docker
// compatibility socket, so a fresh clone must re-establish those Seed-provided
// services after the guest systemd bus is available.
func (p *SandboxProvider) ensureSeedEnvironmentRuntimeReady(ctx context.Context, ref string) error {
	if p == nil || p.Runtime == nil || p.runner == nil || strings.TrimSpace(ref) == "" {
		return core.ErrInvalidArgument
	}

	timeout := p.cleanupTimeout
	if timeout <= 0 {
		timeout = defaultCleanupTimeout
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"exec", ref, "--project", p.project, "--",
		"systemctl", "start", "containerd.service", "hacocoon-docker.socket",
	}
	for {
		result, err := p.runner.Run(readyCtx, "incus", args...)
		if err == nil && result.ExitCode == 0 {
			break
		}
		if !isTransientGuestSystemdStartupFailure(result, err) {
			return fmt.Errorf("start Seed runtime services in %s: %w", ref, commandResultError(result, err))
		}

		select {
		case <-readyCtx.Done():
			return fmt.Errorf("wait for Seed runtime services in %s: %w", ref, readyCtx.Err())
		case <-time.After(seedEnvironmentRuntimeRetryInterval):
		}
	}

	if err := p.expectGuestUnitState(readyCtx, ref, "containerd.service", "active"); err != nil {
		return fmt.Errorf("verify Seed containerd readiness in %s: %w", ref, err)
	}
	if err := p.expectGuestUnitState(readyCtx, ref, "hacocoon-docker.socket", "active"); err != nil {
		return fmt.Errorf("verify Seed Docker socket readiness in %s: %w", ref, err)
	}
	if err := p.expectGuestUnitState(readyCtx, ref, "hacocoon-docker.service", "inactive"); err != nil {
		return fmt.Errorf("Seed Docker daemon started before socket use in %s: %w", ref, err)
	}
	return nil
}

func isTransientGuestSystemdStartupFailure(result host.Result, err error) bool {
	reason := strings.ToLower(strings.TrimSpace(result.Stderr))
	if reason == "" && err != nil {
		reason = strings.ToLower(err.Error())
	}
	if !strings.Contains(reason, "bus") {
		return false
	}
	return strings.Contains(reason, "failed to connect") ||
		strings.Contains(reason, "no such file or directory")
}
