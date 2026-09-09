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
	return p.createEnvironment(ctx, spec, nil)
}

// CreateEnvironmentWithReceipt records exact runtime ownership immediately after
// Incus init, before configuring devices, limits, networking or starting the guest.
// After the receipt callback is invoked, the caller owns failure cleanup.
func (p *SandboxProvider) CreateEnvironmentWithReceipt(ctx context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if record == nil || !core.ValidEnvironmentInstanceID(spec.InstanceID) {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	return p.createEnvironment(ctx, spec, record)
}

func (p *SandboxProvider) createEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if p == nil || p.BaseProvider == nil || p.Runtime == nil || spec.Name == "" || spec.WorkspacePath == "" {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	identityArgs, err := environmentIdentityArgs(spec.InstanceID)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if spec.TemporaryWorkspace && (!core.IsTemporaryWorkspacePath(spec.WorkspacePath) || spec.ReadOnly) {
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
	if err := p.ensureRoutedSandboxHost(ctx); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("ensure Hacocoon routed sandbox substrate: %w", err)
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
	initArgs = append(initArgs, identityArgs...)
	if _, err := p.runner.Run(ctx, "incus", initArgs...); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("init isolated Incus environment %s: %w", ref, err)
	}
	base := resolved.ref
	created := core.EnvironmentRuntime{Ref: ref, Base: &base, Resources: resources}
	cleanup := func(cause error) (core.EnvironmentRuntime, error) {
		// The receipt path delegates cleanup to the canonical lifecycle owner,
		// which already knows this exact resource. Do not delete twice here.
		if record != nil {
			return created, cause
		}

		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.cleanupTimeout)
		defer cancel()
		_, cleanupErr := p.runner.Run(cleanupCtx, "incus", "delete", ref, "--project", p.project, "--force")
		if cleanupErr == nil {
			if guardErr := p.removeRoutedSandboxSourceGuard(cleanupCtx, ref); guardErr != nil {
				return core.EnvironmentRuntime{}, errors.Join(cause, fmt.Errorf("cleanup routed source guard for %s: %w", ref, guardErr), core.ErrRecoveryRequired)
			}
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
		if guardErr := p.removeRoutedSandboxSourceGuard(cleanupCtx, ref); guardErr != nil {
			return core.EnvironmentRuntime{}, errors.Join(cause, fmt.Errorf("cleanup routed source guard for absent %s: %w", ref, guardErr), core.ErrRecoveryRequired)
		}
		return core.EnvironmentRuntime{}, cause
	}

	if record != nil {
		if err := record(created); err != nil {
			return cleanup(err)
		}
	}
	if err := p.configureSandboxEnvironment(ctx, ref, spec, resources, resolved.usesSeed); err != nil {
		return cleanup(err)
	}
	if resolved.built {
		if err := p.renewGuestSSHIdentity(ctx, ref); err != nil {
			return cleanup(err)
		}
	}
	return created, nil
}

func (p *SandboxProvider) addSandboxNIC(ctx context.Context, ref string) error {
	return p.addRoutedSandboxNIC(ctx, ref)
}

// DeleteEnvironment removes the Incus instance before dropping its source
// identity guard. The guard therefore remains fail-closed for as long as a
// guest could still be running on the host-side veth.
func (p *SandboxProvider) DeleteEnvironment(ctx context.Context, ref string) error {
	if p == nil || p.Runtime == nil {
		return core.ErrInvalidArgument
	}
	deleteErr := p.Runtime.DeleteEnvironment(ctx, ref)
	if deleteErr != nil && !errors.Is(deleteErr, core.ErrNotFound) {
		return deleteErr
	}
	guardErr := p.removeRoutedSandboxSourceGuard(ctx, ref)
	if guardErr != nil {
		if deleteErr != nil {
			return errors.Join(deleteErr, fmt.Errorf("cleanup routed source guard for %s: %w", ref, guardErr), core.ErrRecoveryRequired)
		}
		return errors.Join(fmt.Errorf("cleanup routed source guard for %s: %w", ref, guardErr), core.ErrRecoveryRequired)
	}
	return deleteErr
}

func (p *SandboxProvider) SupportsTemporaryWorkspace() bool { return true }

func (p *SandboxProvider) addWorkspaceDevice(ctx context.Context, ref string, spec core.EnvironmentRuntimeSpec) error {
	if spec.TemporaryWorkspace {
		if !core.IsTemporaryWorkspacePath(spec.WorkspacePath) || spec.ReadOnly {
			return core.ErrInvalidArgument
		}
		return nil
	}
	if strings.HasPrefix(spec.WorkspacePath, "managed:") {
		if p.managedWorkspace == nil {
			return core.ErrUnsupported
		}
		attachments, err := p.managedWorkspace(ctx, spec.WorkspacePath)
		if err != nil {
			return err
		}
		if len(attachments) == 0 {
			return core.ErrIncompatibleState
		}
		for _, mount := range attachments {
			if !validWorkspaceAttachment(mount) {
				return core.ErrIncompatibleState
			}
			args := []string{"config", "device", "add", ref, mount.Device, "disk", "pool=" + mount.Pool, "source=" + mount.Volume, "path=" + mount.Path, "--project", p.project}
			if spec.ReadOnly {
				args = append(args, "readonly=true")
			}
			if _, err := p.runner.Run(ctx, "incus", args...); err != nil {
				return err
			}
		}
		return nil
	}
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
