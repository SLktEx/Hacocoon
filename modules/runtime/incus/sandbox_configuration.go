package incus

import (
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
)

// configureSandboxEnvironment is the single post-creation security/materialization
// path for new and restored Environments. The caller owns durable creation and
// cleanup; restoration records its identity before invoking this fallible phase.
func (p *SandboxProvider) configureSandboxEnvironment(ctx context.Context, ref string, spec core.EnvironmentRuntimeSpec, resources core.ResourceBudget, nested bool) error {
	// Environment networking is an authorization boundary. Each Environment
	// receives its own point-to-point routed veth and never joins a shared L2.
	// An exact inet/nft source guard is installed before start; rp_filter is
	// retained as defense-in-depth and verified after start.
	if err := p.addSandboxNIC(ctx, ref); err != nil {
		return fmt.Errorf("materialize sandbox NIC in %s: %w", ref, err)
	}

	if err := p.setAndVerifyConfig(ctx, ref, managedEnvironmentMarkerKey, managedEnvironmentMarkerValue); err != nil {
		return fmt.Errorf("mark managed Incus Environment for trusted Seed harvest: %w", err)
	}

	if nested {
		if err := p.configureNestedOCIInstance(ctx, ref); err != nil {
			return fmt.Errorf("configure nested OCI support for Seed environment: %w", err)
		}
	}

	if err := p.applyResourceBudget(ctx, ref, resources); err != nil {
		return err
	}

	if err := p.addWorkspaceDevice(ctx, ref, spec); err != nil {
		return err
	}
	if !spec.ResourceMaintenance {
		if err := p.attachPersistentResource(ctx, ref, spec.PersistentResource); err != nil {
			return err
		}
	}
	if result, err := p.runner.Run(ctx, "incus", "start", ref, "--project", p.project); err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason == "" {
			reason = err.Error()
		}
		return fmt.Errorf("start Incus environment %s: %s: %w", ref, reason, err)
	}
	if err := p.verifyRoutedSandboxAntiSpoof(ctx, ref); err != nil {
		return fmt.Errorf("verify routed sandbox anti-spoofing for %s: %w", ref, err)
	}
	if err := p.provisionEnvironmentDNS(ctx, ref); err != nil {
		return err
	}
	if spec.ResourceMaintenance {
		if err := p.prepareResourceMaintenance(ctx, ref); err != nil {
			return err
		}
		if err := p.provisionMaintenanceTooling(ctx, ref, spec.InstanceID); err != nil {
			return err
		}
		if err := p.attachPersistentResource(ctx, ref, spec.PersistentResource); err != nil {
			return err
		}
		if err := p.startContainerdMaintenance(ctx, ref, spec.InstanceID, spec.PersistentResource); err != nil {
			return err
		}
	} else if spec.PersistentResource.ID != "" {
		if _, err := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "/bin/sh", "-c", persistentOCIConfiguration); err != nil {
			return fmt.Errorf("configure Environment-local OCI data roots: %w", err)
		}
	}
	if spec.TemporaryWorkspace {
		if _, err := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "/bin/sh", "-ec", "test ! -L /workspace; mkdir -p /workspace"); err != nil {
			return fmt.Errorf("prepare temporary Workspace: %w", err)
		}
	}
	if !spec.ReadOnly {
		result, err := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "test", "-w", "/workspace")
		if err != nil {
			reason := strings.TrimSpace(result.Stderr)
			if reason == "" {
				reason = err.Error()
			}
			return errors.Join(
				fmt.Errorf("workspace %q is not writable from unprivileged environment %s: %s", spec.WorkspacePath, ref, reason),
				core.ErrUnsupported,
			)
		}
	}
	return nil
}
