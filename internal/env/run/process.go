package run

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type processLifecycle interface {
	ExecRunStream(context.Context, string, string, core.ProcessRequest, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error)
}

// RunStream uses the same creation, recovery, ownership and cleanup as Run.
func (s *Service) RunStream(ctx context.Context, spec Spec, tty bool, stdin io.Reader, stdout, stderr io.Writer) (Result, error) {
	if s == nil || len(spec.Argv) == 0 || stdin == nil || stdout == nil || stderr == nil {
		return Result{}, core.ErrInvalidArgument
	}
	if err := core.ValidateProcessRequest(core.ProcessRequest{WorkingDirectory: "/workspace", Argv: spec.Argv, TTY: tty}); err != nil {
		return Result{}, err
	}
	process, ok := s.environments.(processLifecycle)
	if !ok {
		return Result{}, core.ErrUnsupported
	}
	out := &processCounter{writer: stdout}
	diagnostic := &processCounter{writer: stderr}
	return s.run(ctx, spec, core.PersistentResourceRef{}, func(ctx context.Context, env core.Environment, instance string) (core.ExecutionResult, error) {
		result, err := process.ExecRunStream(ctx, env.Name, instance, core.ProcessRequest{WorkingDirectory: "/workspace", Argv: append([]string(nil), spec.Argv...), TTY: tty}, stdin, out, diagnostic)
		result.Stdout, result.Stderr = "", ""
		result.StdoutBytes, result.StderrBytes = out.bytes.Load(), diagnostic.bytes.Load()
		return result, err
	})
}

type processCounter struct {
	writer io.Writer
	bytes  atomic.Int64
}

func (w *processCounter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.bytes.Add(int64(n))
	}
	return n, err
}
