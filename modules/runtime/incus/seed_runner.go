package incus

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// CloneSandboxProviderWithRunner creates a Seed/backend view of an existing
// provider with a different command runner. Runtime storage state, Base source
// configuration, and Seed resolution are retained, while ordinary Environment
// operations can keep using the original Physical-Host Incus runner.
func CloneSandboxProviderWithRunner(provider *SandboxProvider, runner host.Runner) (*SandboxProvider, error) {
	if provider == nil || provider.BaseProvider == nil || provider.Runtime == nil || runner == nil {
		return nil, core.ErrInvalidArgument
	}
	runtimeClone := *provider.Runtime
	runtimeClone.runner = runner
	baseClone := *provider.BaseProvider
	baseClone.Runtime = &runtimeClone
	return &SandboxProvider{BaseProvider: &baseClone}, nil
}
