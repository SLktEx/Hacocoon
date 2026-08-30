package incus

import (
	"context"
	"errors"
	"io"
	"os/exec"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ShellEnvironmentStream opens a shell through Incus while attaching it to
// caller-provided streams. Force interactive mode because controller streams
// are sockets rather than terminal file descriptors, so Incus cannot infer PTY
// mode from the controller process stdio.
func (r *Runtime) ShellEnvironmentStream(ctx context.Context, ref string, stdin io.Reader, stdout, stderr io.Writer) error {
	if r == nil || ref == "" || stdin == nil || stdout == nil || stderr == nil {
		return core.ErrInvalidArgument
	}
	_, err := r.execInteractiveStream(ctx, ref, []string{"/bin/bash"}, stdin, stdout, stderr)
	return err
}

func (r *Runtime) execInteractiveStream(ctx context.Context, ref string, argv []string, stdin io.Reader, stdout, stderr io.Writer) (core.ExecResult, error) {
	if len(argv) == 0 {
		return core.ExecResult{}, core.ErrInvalidArgument
	}
	args := append([]string{"exec", ref, "--project", r.project, "--force-interactive", "--"}, argv...)
	cmd := exec.CommandContext(ctx, "incus", args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return core.ExecResult{ExitCode: 0}, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return core.ExecResult{ExitCode: exit.ExitCode()}, err
	}
	return core.ExecResult{ExitCode: -1}, err
}
