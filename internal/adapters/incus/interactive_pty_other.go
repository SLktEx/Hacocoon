//go:build !linux

package incus

import (
	"context"
	"io"
	"os/exec"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// The supported Incus controller runs on Linux, including for Windows/WSL
// clients. Never claim size forwarding from an unsupported controller platform.
func runSizedInteractiveCommand(context.Context, *exec.Cmd, io.Reader, core.TerminalMetadata) error {
	return core.ErrUnsupported
}
