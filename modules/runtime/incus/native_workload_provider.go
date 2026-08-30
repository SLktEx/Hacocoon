package incus

import (
	"context"
	"errors"
	"fmt"
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
