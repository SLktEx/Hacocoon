package incus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// SandboxProvider composes v0.11 immutable Base resolution with v0.12
// creation-time resource enforcement while retaining Runtime's existing client
// and lifecycle methods through embedding.
type SandboxProvider struct {
	*BaseProvider
}

func NewSandboxProvider(runtime *Runtime, options ...BaseProviderOption) (*SandboxProvider, error) {
	base, err := NewBaseProvider(runtime, options...)
	if err != nil {
		return nil, err
	}
	return &SandboxProvider{BaseProvider: base}, nil
}

func (*SandboxProvider) SupportsFiniteResourceBudgets() bool { return true }

func (p *SandboxProvider) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if p == nil || p.BaseProvider == nil || p.Runtime == nil || spec.Name == "" || spec.WorkspacePath == "" {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	resources, err := core.ResolveResourceBudget(spec.Resources)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	resolved, err := p.resolveBase(ctx, spec.Base)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err := p.ensureProject(ctx); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("ensure Incus project: %w", err)
	}
	rootPool, err := p.defaultRootPool(ctx)
	if err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("resolve isolated root storage: %w", err)
	}
	if err := p.ensureSandboxNetwork(ctx); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("ensure Hacocoon sandbox network: %w", err)
	}

	ref := "haco-" + spec.Name
	profileConfig, err := p.sandboxProfileConfig(ctx)
	if err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("resolve Hacocoon sandbox proxy configuration: %w", err)
	}
	initArgs := []string{
		"init", resolved.pinnedSource, ref,
		"--project", p.project,
		"--no-profiles",
		"--storage", rootPool,
	}
	configKeys := make([]string, 0, len(profileConfig))
	for key := range profileConfig {
		configKeys = append(configKeys, key)
	}
	sort.Strings(configKeys)
	for _, key := range configKeys {
		initArgs = append(initArgs, "--config", key+"="+profileConfig[key])
	}
	if _, err := p.runner.Run(ctx, "incus", initArgs...); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("init isolated Incus environment %s: %w", ref, err)
	}
	cleanup := func(cause error) (core.EnvironmentRuntime, error) {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.cleanupTimeout)
		defer cancel()
		_, cleanupErr := p.runner.Run(cleanupCtx, "incus", "delete", ref, "--project", p.project, "--force")
		if cleanupErr == nil {
			return core.EnvironmentRuntime{}, cause
		}
		if cleanupCtx.Err() != nil {
			return core.EnvironmentRuntime{}, errors.Join(
				cause,
				fmt.Errorf("cleanup Incus environment %s: %w", ref, cleanupErr),
				core.ErrRecoveryRequired,
			)
		}
		exists, inspectErr := p.environmentExists(cleanupCtx, ref)
		if inspectErr != nil {
			return core.EnvironmentRuntime{}, errors.Join(
				cause,
				fmt.Errorf("cleanup Incus environment %s: %w", ref, cleanupErr),
				fmt.Errorf("confirm Incus cleanup state for %s: %w", ref, inspectErr),
				core.ErrRecoveryRequired,
			)
		}
		if exists {
			return core.EnvironmentRuntime{}, errors.Join(
				cause,
				fmt.Errorf("cleanup Incus environment %s: %w", ref, cleanupErr),
				core.ErrRecoveryRequired,
			)
		}
		return core.EnvironmentRuntime{}, cause
	}

	// Environment networking is an authorization boundary. Do not rely on a
	// profile inherited across Incus project boundaries: materialize the managed
	// NIC directly on the instance before it can start.
	if err := p.addSandboxNIC(ctx, ref); err != nil {
		return cleanup(fmt.Errorf("materialize sandbox NIC in %s: %w", ref, err))
	}

	if err := p.setAndVerifyConfig(ctx, ref, managedEnvironmentMarkerKey, managedEnvironmentMarkerValue); err != nil {
		return cleanup(fmt.Errorf("mark managed Incus Environment for trusted Seed harvest: %w", err))
	}

	if resolved.usesSeed {
		if err := p.configureNestedOCIInstance(ctx, ref); err != nil {
			return cleanup(fmt.Errorf("configure nested OCI support for Seed environment: %w", err))
		}
	}

	if err := p.applyResourceBudget(ctx, ref, resources); err != nil {
		return cleanup(err)
	}

	if err := p.addWorkspaceDevice(ctx, ref, spec); err != nil {
		return cleanup(err)
	}
	if result, err := p.runner.Run(ctx, "incus", "start", ref, "--project", p.project); err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason == "" {
			reason = err.Error()
		}
		return cleanup(fmt.Errorf("start Incus environment %s: %s: %w", ref, reason, err))
	}
	if !spec.ReadOnly {
		result, err := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "test", "-w", "/workspace")
		if err != nil {
			reason := strings.TrimSpace(result.Stderr)
			if reason == "" {
				reason = err.Error()
			}
			return cleanup(errors.Join(
				fmt.Errorf("workspace %q is not writable from unprivileged environment %s: %s", spec.WorkspacePath, ref, reason),
				core.ErrUnsupported,
			))
		}
	}
	base := resolved.ref
	return core.EnvironmentRuntime{Ref: ref, Base: &base, Resources: resources}, nil
}

func (p *SandboxProvider) addSandboxNIC(ctx context.Context, ref string) error {
	args := []string{"config", "device", "add", ref, "eth0", "nic"}
	for _, key := range []string{
		"name",
		"network",
		"security.ipv4_filtering",
		"security.ipv6_filtering",
		"security.mac_filtering",
		"security.port_isolation",
	} {
		args = append(args, key+"="+sandboxNIC[key])
	}
	args = append(args, "--project", p.project)
	result, err := p.runner.Run(ctx, "incus", args...)
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason != "" {
			return fmt.Errorf("add managed sandbox NIC: %s: %w", reason, err)
		}
		return fmt.Errorf("add managed sandbox NIC: %w", err)
	}
	return nil
}

func (p *SandboxProvider) addWorkspaceDevice(ctx context.Context, ref string, spec core.EnvironmentRuntimeSpec) error {
	deviceArgs := []string{
		"config", "device", "add", ref, "workspace", "disk",
		"source=" + spec.WorkspacePath,
		"path=/workspace",
	}
	if spec.ReadOnly {
		deviceArgs = append(deviceArgs, "readonly=true")
	} else {
		uid, gid, ownerErr := workspaceOwnerIDs(spec.WorkspacePath)
		if ownerErr == nil && uid != 0 && gid != 0 && workspaceOwnerIDsMappable(uid, gid) {
			// Keep the container unprivileged, but map only the owner identity of
			// the explicitly leased host workspace to root inside the sandbox.
			// This lets an agent running as container root edit an ordinary
			// user-owned checkout without granting a broad host UID/GID range.
			idmap := fmt.Sprintf("uid %d 0\ngid %d 0", uid, gid)
			if err := p.setAndVerifyConfig(ctx, ref, "raw.idmap", idmap); err != nil {
				return fmt.Errorf("map workspace owner into unprivileged environment %s: %w", ref, err)
			}
		} else {
			// Preserve the existing idmapped-mount path when the workspace owner
			// is not available in root's subordinate ID ranges. The post-start
			// write probe remains fail-closed if this is insufficient.
			deviceArgs = append(deviceArgs, "shift=true")
		}
	}
	deviceArgs = append(deviceArgs, "--project", p.project)
	if _, err := p.runner.Run(ctx, "incus", deviceArgs...); err != nil {
		return fmt.Errorf("mount workspace in %s: %w", ref, err)
	}
	return nil
}

func (p *SandboxProvider) applyResourceBudget(ctx context.Context, ref string, budget core.ResourceBudget) error {
	if budget.CPU.Mode == core.ResourceLimitFinite {
		value := strconv.FormatUint(budget.CPU.Value, 10)
		if err := p.setAndVerifyConfig(ctx, ref, "limits.cpu", value); err != nil {
			return fmt.Errorf("apply CPU resource limit: %w", err)
		}
	}
	if budget.MemoryBytes.Mode == core.ResourceLimitFinite {
		value := strconv.FormatUint(budget.MemoryBytes.Value, 10) + "B"
		if err := p.setAndVerifyConfig(ctx, ref, "limits.memory", value); err != nil {
			return fmt.Errorf("apply memory resource limit: %w", err)
		}
	}
	if budget.PIDs.Mode == core.ResourceLimitFinite {
		value := strconv.FormatUint(budget.PIDs.Value, 10)
		if err := p.setAndVerifyConfig(ctx, ref, "limits.processes", value); err != nil {
			return fmt.Errorf("apply PID resource limit: %w", err)
		}
	}
	if budget.RootBytes.Mode == core.ResourceLimitFinite {
		value := strconv.FormatUint(budget.RootBytes.Value, 10) + "B"
		if _, err := p.runner.Run(ctx, "incus", "config", "device", "set", ref, "root", "size="+value, "--project", p.project); err != nil {
			return errors.Join(fmt.Errorf("apply root-disk resource limit: %w", err), core.ErrUnsupported)
		}
		got, err := p.runner.Run(ctx, "incus", "config", "device", "get", ref, "root", "size", "--project", p.project)
		if err != nil {
			return fmt.Errorf("verify root-disk resource limit: %w", err)
		}
		if strings.TrimSpace(got.Stdout) != value {
			return fmt.Errorf("verify root-disk resource limit: provider returned %q, want %q: %w", strings.TrimSpace(got.Stdout), value, core.ErrIncompatibleState)
		}
	}
	return nil
}

func (p *SandboxProvider) setAndVerifyConfig(ctx context.Context, ref, key, value string) error {
	if _, err := p.runner.Run(ctx, "incus", "config", "set", ref, key+"="+value, "--project", p.project); err != nil {
		return err
	}
	got, err := p.runner.Run(ctx, "incus", "config", "get", ref, key, "--project", p.project)
	if err != nil {
		return err
	}
	if strings.TrimSpace(got.Stdout) != value {
		return fmt.Errorf("provider returned %q for %s, want %q: %w", strings.TrimSpace(got.Stdout), key, value, core.ErrIncompatibleState)
	}
	return nil
}
