package workspace

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/terminalsession"
)

type environmentStreamRuntime interface {
	ShellEnvironmentStream(context.Context, string, io.Reader, io.Writer, io.Writer) error
}

// PrepareShellStream resolves the Environment and provider capability before
// the controller acknowledges a stream. The returned function owns the actual
// long-lived interactive execution.
func (s *Service) PrepareShellStream(ctx context.Context, name string) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	if s == nil {
		return nil, core.ErrInvalidArgument
	}
	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return nil, err
	}
	runtime, ok := s.runtime.(environmentStreamRuntime)
	if !ok {
		return nil, fmt.Errorf("runtime does not support streamed shells: %w", core.ErrUnsupported)
	}
	ref := environment.RuntimeRef
	terminal := core.TerminalMetadataFromContext(ctx)
	resizeSource := terminalsession.ResizeSourceFromContext(ctx)
	return func(runCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) (err error) {
		if stdin == nil || stdout == nil || stderr == nil {
			return core.ErrInvalidArgument
		}
		started := time.Now()
		runCtx = core.WithTerminalMetadata(runCtx, terminal)
		runCtx = terminalsession.WithResizeSource(runCtx, resizeSource)
		runCtx = logging.With(runCtx, "operation", "shell_environment_stream", "environment_id", name)
		logger := logging.FromContext(runCtx).With("component", "core")
		logger.InfoContext(runCtx, "opening streamed environment shell")
		defer func() {
			if err != nil {
				logger.ErrorContext(runCtx, "streamed environment shell failed",
					"duration_ms", time.Since(started).Milliseconds(),
					"error", err,
				)
				return
			}
			logger.InfoContext(runCtx, "streamed environment shell closed", "duration_ms", time.Since(started).Milliseconds())
		}()
		return runtime.ShellEnvironmentStream(runCtx, ref, stdin, stdout, stderr)
	}, nil
}

// ShellStream opens an interactive Environment shell using caller-provided I/O.
// It remains a convenience wrapper for non-controller callers.
func (s *Service) ShellStream(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) error {
	prepared, err := s.PrepareShellStream(ctx, name)
	if err != nil {
		return err
	}
	return prepared(ctx, stdin, stdout, stderr)
}
