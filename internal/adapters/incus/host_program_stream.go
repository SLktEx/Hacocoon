package incus

import (
	"bytes"
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"os/exec"
	"time"
)

// RunTrustedHostPythonStream bounds execution, not total file size. The caller
// validates streamed frames and publishes data only after the final receipt.
func (r *Runtime) RunTrustedHostPythonStream(ctx context.Context, program string, input []byte, out io.Writer) error {
	if len(program) == 0 || len(program) > 65536 || len(input) > 8192 || out == nil {
		return core.ErrInvalidArgument
	}
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	args := r.trustedHostPythonArgs(program, "590s")
	command := exec.CommandContext(ctx, "incus", args...)
	command.Stdin = bytes.NewReader(input)
	command.Stdout = out
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		return fmt.Errorf("trusted Host stream failed: %w", core.ErrRuntimeUnavailable)
	}
	return nil
}
func (r *Runtime) trustedHostPythonArgs(program, lifetime string) []string {
	return []string{"exec", trustedHostName, "--project", r.project, "--cwd", "/root", "--",
		"/usr/bin/systemd-run", "--quiet", "--collect", "--wait", "--pipe", "--service-type=exec",
		"--working-directory=/root", "--property=KillMode=control-group", "--property=TimeoutStopSec=5s", "--property=RuntimeMaxSec=" + lifetime, "--property=MemoryMax=512M", "--property=TasksMax=64",
		"/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/root", "/usr/bin/python3", "-I", "-c", program}
}
