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
	return p.startEnvironment(ctx, ref, "", nil)
}

// The caller supplies the complete aggregate under its canonical lifecycle lock.
// Reference-only resume cannot bypass a native data-binding receipt.
func (p *SandboxProvider) StartEnvironmentWithResources(ctx context.Context, ref, instance string, areas []core.EnvironmentRuntimeAttachment) error {
	if !core.ValidEnvironmentInstanceID(instance) || len(areas) == 0 {
		return core.ErrInvalidArgument
	}
	return p.startEnvironment(ctx, ref, instance, areas)
}

func (p *SandboxProvider) startEnvironment(ctx context.Context, ref, instance string, areas []core.EnvironmentRuntimeAttachment) error {
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
	if err := p.verifyEnvironmentResources(ctx, ref, instance, areas, status.State); err != nil {
		return err
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
	// Restore missing volatile guards only while stopped and after ownership
	// checks. Existing drift is rejected, and all guards precede guest start.
	if err := p.verifyRoutedSandboxAntiSpoofForStart(ctx, ref, status.State == core.EnvironmentStopped); err != nil {
		return err
	}
	// Adopt this boot policy for owned legacy instances only after validation.
	if err := p.setAndVerifyConfig(ctx, ref, "boot.autostart", "false"); err != nil {
		return fmt.Errorf("disable automatic Environment startup: %w", err)
	}
	if status.State == core.EnvironmentRunning {
		return p.provisionEnvironmentDNS(ctx, ref)
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
		err = p.provisionEnvironmentDNS(ctx, ref)
	}
	if err == nil {
		return nil
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	return errors.Join(fmt.Errorf("verify resumed Environment: %w", err), p.StopEnvironment(cleanup, ref))
}
