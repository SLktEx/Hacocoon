package environment

import "github.com/SLktEx/Hacocoon/internal/core"

func validateTemporaryProvider(provider Provider, spec core.EnvironmentRuntimeSpec) error {
	if !spec.TemporaryWorkspace {
		return nil
	}
	supported, ok := provider.(interface{ SupportsTemporaryWorkspace() bool })
	if !ok || !supported.SupportsTemporaryWorkspace() {
		return core.ErrUnsupported
	}
	if !core.IsTemporaryWorkspacePath(spec.WorkspacePath) || spec.ReadOnly {
		return core.ErrInvalidArgument
	}
	return nil
}
