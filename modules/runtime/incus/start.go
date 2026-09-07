package incus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// StartEnvironment is only exposed by the sandbox provider: resuming a saved
// instance must enforce the same network boundary as creating it.
func (p *SandboxProvider) StartEnvironment(ctx context.Context, ref string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	marker, err := p.runner.Run(ctx, "incus", "config", "get", ref, managedEnvironmentMarkerKey, "--project", p.project)
	if err != nil {
		return err
	}
	if strings.TrimSpace(marker.Stdout) != managedEnvironmentMarkerValue {
		return core.ErrIncompatibleState
	}
	status, err := p.InspectEnvironment(ctx, ref)
	if err != nil {
		return err
	}
	if status.State != core.EnvironmentRunning && status.State != core.EnvironmentStopped {
		return core.ErrIncompatibleState
	}
	owner, err := p.runner.Run(ctx, "incus", "network", "get", environmentBridgeName(ref), environmentNetworkOwnerKey, "--project", sandboxBridgeResourceProject)
	if err != nil {
		return err
	}
	if strings.TrimSpace(owner.Stdout) != environmentNetworkOwnerValue {
		return core.ErrIncompatibleState
	}
	isolation, err := p.runner.Run(ctx, "incus", "config", "device", "get", ref, "eth0", "security.port_isolation", "--project", p.project)
	if err != nil {
		return err
	}
	if strings.TrimSpace(isolation.Stdout) != "true" {
		return core.ErrIncompatibleState
	}
	if err := p.ensureRoutedSandboxHost(ctx); err != nil {
		return err
	}
	// Missing or drifted guards fail closed; restarting must never silently
	// weaken isolation or start first and attempt to fix its firewall afterwards.
	if err := p.verifyRoutedSandboxAntiSpoof(ctx, ref); err != nil {
		return err
	}
	if status.State == core.EnvironmentRunning {
		return nil
	}
	if err := p.Start(ctx, ref); err != nil {
		return err
	}
	status, err = p.InspectEnvironment(ctx, ref)
	if err == nil && status.State != core.EnvironmentRunning {
		err = core.ErrIncompatibleState
	}
	if err == nil {
		err = p.verifyRoutedSandboxAntiSpoof(ctx, ref)
	}
	if err == nil {
		return nil
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	return errors.Join(fmt.Errorf("verify resumed Environment: %w", err), p.StopEnvironment(cleanup, ref))
}
