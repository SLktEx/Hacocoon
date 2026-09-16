package environment

import (
	"context"
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type ProcessProvider interface {
	ExecEnvironmentStream(context.Context, string, core.ProcessRequest, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error)
}

type ShellStreamProvider interface {
	ShellEnvironmentStream(context.Context, string, io.Reader, io.Writer, io.Writer) error
}

func (r *Router) ShellEnvironmentStream(ctx context.Context, rawRef string, stdin io.Reader, stdout, stderr io.Writer) error {
	provider, ref, err := r.resolve(rawRef)
	if err != nil {
		return err
	}
	shell, ok := provider.(ShellStreamProvider)
	if !ok {
		return core.ErrUnsupported
	}
	return shell.ShellEnvironmentStream(ctx, ref, stdin, stdout, stderr)
}

func (r *Router) ExecEnvironmentStream(ctx context.Context, rawRef string, request core.ProcessRequest, stdin io.Reader, stdout, stderr io.Writer) (core.ExecutionResult, error) {
	provider, ref, err := r.resolve(rawRef)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	process, ok := provider.(ProcessProvider)
	if !ok {
		return core.ExecutionResult{}, core.ErrUnsupported
	}
	return process.ExecEnvironmentStream(ctx, ref, request, stdin, stdout, stderr)
}
