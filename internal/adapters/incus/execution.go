package incus

import (
	"context"
	"os/exec"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func (r *Runtime) SupportsWorkingDirectory() bool { return true }
func (r *Runtime) SupportsStdin() bool            { _, ok := r.runner.(host.InputRunner); return ok }

func (r *Runtime) ExecEnvironment(ctx context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if len(req.Argv) == 0 || len(req.Stdin) > core.MaxExecutionInputBytes {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	args, err := r.executionArgs(ref, req.WorkingDirectory, req.Argv)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	var result host.Result
	if req.Stdin != nil {
		runner, ok := r.runner.(host.InputRunner)
		if !ok {
			return core.ExecutionResult{}, core.ErrUnsupported
		}
		result, err = runner.RunWithInput(ctx, req.Stdin, "incus", args...)
	} else {
		result, err = r.runner.Run(ctx, "incus", args...)
	}
	return core.ExecutionResult{
		ExitCode:        result.ExitCode,
		StdoutTruncated: result.StdoutTruncated, StderrTruncated: result.StderrTruncated, StdoutBytes: result.StdoutBytes, StderrBytes: result.StderrBytes,
		Stdout: result.Stdout,
		Stderr: result.Stderr,
	}, err
}

func (r *Runtime) ShellEnvironment(ctx context.Context, ref string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	return r.execInteractive(ctx, ref, []string{"/bin/bash"})
}

func (r *Runtime) execInteractive(ctx context.Context, ref string, argv []string) error {
	args := append([]string{"exec", ref, "--project", r.project, "--"}, argv...)
	cmd := exec.CommandContext(ctx, "incus", args...)
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}
