package incus

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Runtime) ExecEnvironmentStream(ctx context.Context, ref string, request core.ProcessRequest, stdin io.Reader, stdout, stderr io.Writer) (core.ExecutionResult, error) {
	if r == nil || stdin == nil || stdout == nil || stderr == nil {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	if err := core.ValidateProcessRequest(request); err != nil {
		return core.ExecutionResult{}, err
	}
	args, err := r.executionArgs(ref, request.WorkingDirectory, request.Argv)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	mode := "--force-noninteractive"
	if request.TTY {
		mode = "--force-interactive"
	}
	// Insert only trusted adapter options before the command's explicit separator.
	options := []string{mode}
	metadata := core.TerminalMetadataFromContext(ctx)
	if request.TTY {
		if metadata.Columns < 1 || metadata.Rows < 1 {
			return core.ExecutionResult{}, core.ErrInvalidArgument
		}
		if metadata.Term != "" {
			options = append(options, "--env", "TERM="+metadata.Term)
		}
		if metadata.ColorTerm != "" {
			options = append(options, "--env", "COLORTERM="+metadata.ColorTerm)
		}
	}
	separator := len(args) - len(request.Argv) - 1
	args = append(append(append([]string(nil), args[:separator]...), options...), args[separator:]...)
	cmd := exec.CommandContext(ctx, "incus", args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = 5 * time.Second
	if request.TTY {
		err = runSizedInteractiveCommand(ctx, cmd, stdin, metadata)
	} else {
		err = runInteractiveCommand(cmd, stdin)
	}
	if err == nil {
		return core.ExecutionResult{}, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return core.ExecutionResult{ExitCode: exit.ExitCode()}, err
	}
	return core.ExecutionResult{ExitCode: -1}, err
}

func (r *Runtime) executionArgs(ref, cwd string, argv []string) ([]string, error) {
	if err := validateManagedInstanceRef(ref); err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, core.ErrInvalidArgument
	}
	args := []string{"exec", ref, "--project", r.project}
	if cwd != "" {
		if !strings.HasPrefix(cwd, "/") || strings.ContainsAny(cwd, "\x00\r\n") {
			return nil, core.ErrInvalidArgument
		}
		args = append(args, "--cwd", cwd)
	}
	return append(append(args, "--"), argv...), nil
}
