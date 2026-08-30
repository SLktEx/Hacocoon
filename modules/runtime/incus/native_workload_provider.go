package incus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// NativeWorkloadProvider keeps the existing system-container Environment
// provider intact and layers Incus-native OCI workload access on top. This
// preserves the legacy Seed implementation as a fallback while new packaged
// Environments use the scoped nerdctl shim instead of an instance-local
// containerd daemon.
type NativeWorkloadProvider struct {
	*SandboxProvider
}

func NewNativeWorkloadProvider(provider *SandboxProvider) (*NativeWorkloadProvider, error) {
	if provider == nil || provider.Runtime == nil {
		return nil, core.ErrInvalidArgument
	}
	return &NativeWorkloadProvider{SandboxProvider: provider}, nil
}

func (p *NativeWorkloadProvider) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if p == nil || p.SandboxProvider == nil || p.Runtime == nil {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	created, err := p.SandboxProvider.CreateEnvironment(ctx, spec)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err := p.Runtime.EnsureEnvironmentWorkloadIntegration(ctx, spec.Name, created.Ref); err == nil {
		return created, nil
	} else {
		cleanupTimeout := p.Runtime.cleanupTimeout
		if cleanupTimeout <= 0 {
			cleanupTimeout = 30 * time.Second
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		cleanupErr := p.Runtime.DeleteEnvironment(cleanupCtx, created.Ref)
		if cleanupErr == nil || errors.Is(cleanupErr, core.ErrNotFound) {
			return core.EnvironmentRuntime{}, fmt.Errorf("configure Incus-native workload access for %s: %w", created.Ref, err)
		}
		return core.EnvironmentRuntime{}, errors.Join(
			fmt.Errorf("configure Incus-native workload access for %s: %w", created.Ref, err),
			fmt.Errorf("cleanup Environment after workload integration failure: %w", cleanupErr),
			core.ErrRecoveryRequired,
		)
	}
}

// DeleteEnvironment removes sibling OCI workloads only when this Environment
// actually carries the scoped workload proxy device. Legacy/source-tree paths
// without the packaged companion keep the previous deletion behavior intact.
func (p *NativeWorkloadProvider) DeleteEnvironment(ctx context.Context, ref string) error {
	if p == nil || p.SandboxProvider == nil || p.Runtime == nil {
		return core.ErrRuntimeUnavailable
	}
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	environment := strings.TrimPrefix(ref, "haco-")
	canonical, err := ManagedEnvironmentRef(environment)
	if err != nil || canonical != ref {
		return core.ErrInvalidArgument
	}

	_, _, nativeAvailable, err := workloadShimSource()
	if err != nil {
		return fmt.Errorf("resolve native workload companion before deleting Environment %s: %w", ref, err)
	}
	if !nativeAvailable {
		return p.SandboxProvider.DeleteEnvironment(ctx, ref)
	}

	integrated, err := p.hasNativeWorkloadIntegration(ctx, environment, ref)
	if err != nil {
		return err
	}
	if integrated {
		workloads, err := p.Runtime.ListWorkloads(ctx, environment)
		if err != nil && !errors.Is(err, core.ErrNotFound) {
			return fmt.Errorf("list OCI workloads before deleting Environment %s: %w", ref, err)
		}
		var cleanupErrors []error
		for _, workload := range workloads {
			if deleteErr := p.Runtime.DeleteWorkload(ctx, environment, workload.Name); deleteErr != nil && !errors.Is(deleteErr, core.ErrNotFound) {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("delete workload %s: %w", workload.Name, deleteErr))
			}
		}
		if len(cleanupErrors) > 0 {
			return errors.Join(append(cleanupErrors, core.ErrRecoveryRequired)...)
		}
	}
	return p.SandboxProvider.DeleteEnvironment(ctx, ref)
}

func (p *NativeWorkloadProvider) hasNativeWorkloadIntegration(ctx context.Context, environment, ref string) (bool, error) {
	result, err := p.runner.Run(ctx, "incus", "config", "device", "list", ref, "--project", p.project)
	if err != nil {
		return false, fmt.Errorf("inspect Environment workload integration: %w", err)
	}
	found := false
	for _, name := range strings.Fields(result.Stdout) {
		if name == environmentWorkloadDevice {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	hostSocket, err := WorkloadBrokerSocketPath(environment)
	if err != nil {
		return false, err
	}
	if err := p.Runtime.verifyEnvironmentWorkloadDevice(ctx, ref, hostSocket); err != nil {
		return false, fmt.Errorf("verify Environment workload integration before delete: %w", err)
	}
	return true, nil
}
