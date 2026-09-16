//go:build !linux

package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (*SandboxProvider) SupportsEnvironmentResources() bool { return false }
func (*SandboxProvider) verifyEnvironmentDataPaths(context.Context, string, []core.EnvironmentRuntimeAttachment, []WorkspaceAttachment) error {
	return core.ErrUnsupported
}
