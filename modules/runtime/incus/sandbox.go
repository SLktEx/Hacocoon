package incus

import (
	"context"
	"errors"
	"fmt"
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

func NewSandboxProvider(runtime *Runtime) (*SandboxProvider, error) {
	base, err := NewBaseProvider(runtime)
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
	if _, err := p.runner.Run(ctx, "incus", "init", resolved.pinnedSource, ref,
		"--project", p.project,
		"--profile", sandboxProfile,
		"--storage", rootPool,
	); err != nil {
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

	if err := p.applyResourceBudget(ctx, ref, resources); err != nil {
		return cleanup(err)
	}

	deviceArgs := []string{
		"config", "device", "add", ref, "workspace", "disk",
		"source=" + spec.WorkspacePath,
		"path=/workspace",
	}
	if spec.ReadOnly {
		deviceArgs = append(deviceArgs, "readonly=true")
	} else {
		deviceArgs = append(deviceArgs, "shift=true")
	}
	deviceArgs = append(deviceArgs, "--project", p.project)
	if _, err := p.runner.Run(ctx, "incus", deviceArgs...); err != nil {
		return cleanup(fmt.Errorf("mount workspace in %s: %w", ref, err))
	}
	if _, err := p.runner.Run(ctx, "incus", "start", ref, "--project", p.project); err != nil {
		return cleanup(fmt.Errorf("start Incus environment %s: %w", ref, err))
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
