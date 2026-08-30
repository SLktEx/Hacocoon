package workspace

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

type environmentStreamRuntime interface {
	ShellEnvironmentStream(context.Context, string, io.Reader, io.Writer, io.Writer) error
}

// ShellStream opens an interactive Environment shell using caller-provided I/O.
// It is intentionally separate from Shell so existing direct/local clients keep
// their current stdio behavior while controller-mediated clients can bridge a
// stream without exposing the Incus socket.
func (s *Service) ShellStream(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	if s == nil || stdin == nil || stdout == nil || stderr == nil {
		return core.ErrInvalidArgument
	}
	started := time.Now()
	ctx = logging.With(ctx, "operation", "shell_environment_stream", "environment_id", name)
	logger := logging.FromContext(ctx).With("component", "core")
	logger.InfoContext(ctx, "opening streamed environment shell")
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "streamed environment shell failed",
				"duration_ms", time.Since(started).Milliseconds(),
				"error", err,
			)
			return
		}
		logger.InfoContext(ctx, "streamed environment shell closed", "duration_ms", time.Since(started).Milliseconds())
	}()

	environment, err := s.store.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	runtime, ok := s.runtime.(environmentStreamRuntime)
	if !ok {
		return fmt.Errorf("runtime does not support streamed shells: %w", core.ErrUnsupported)
	}
	return runtime.ShellEnvironmentStream(ctx, environment.RuntimeRef, stdin, stdout, stderr)
}
