package clientforward

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

// The parent owns a real pipe, not an exec stdin copy goroutine. Parent death
// closes its write handle even when no bytes are being written. Cancellation
// closes the lease first; WaitDelay kills only the exact unresponsive child.
func runCompanion(ctx context.Context, cmd *exec.Cmd, request delegation, out, diagnostic io.Writer) (code int, resultErr error) {
	started, stage := time.Now(), "pipe"
	defer func() {
		if resultErr != nil {
			recordCompanionFailure(ctx, stage, resultErr, time.Since(started))
		}
	}()
	reader, writer, err := os.Pipe()
	if err != nil {
		return 1, err
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = reader, out, diagnostic
	cmd.Cancel = func() error { return writer.Close() }
	cmd.WaitDelay = 10 * time.Second
	stage = "start"
	if err := cmd.Start(); err != nil {
		return 1, err
	}
	_ = reader.Close()
	stage = "prepare"
	preparation := time.AfterFunc(10*time.Second, func() { _ = writer.Close(); _ = cmd.Process.Kill() })
	err = writeDelegation(writer, request)
	if !preparation.Stop() || err != nil {
		_ = writer.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return 1, errors.New("windows tunnel preparation failed")
	}
	stage = "wait"
	err = cmd.Wait()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 0 {
		return exit.ExitCode(), nil
	}
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return 0, nil
	}
	return 1, err
}

func recordCompanionFailure(ctx context.Context, stage string, err error, elapsed time.Duration) {
	// Classify only; subprocess errors can contain private paths, argv or output.
	reason := "other"
	var exit *exec.ExitError
	switch {
	case errors.Is(err, exec.ErrWaitDelay):
		reason = "wait_delay"
	case errors.As(err, &exit):
		reason = "exit"
		if exit.ExitCode() < 0 {
			reason = "signaled"
		}
	case errors.Is(err, context.DeadlineExceeded):
		reason = "timeout"
	case errors.Is(err, context.Canceled):
		reason = "canceled"
	case errors.Is(err, os.ErrClosed), errors.Is(err, io.ErrClosedPipe):
		reason = "pipe_closed"
	}
	cancellation := "active"
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		cancellation = "timeout"
	} else if ctx.Err() != nil {
		cancellation = "canceled"
	}
	logging.FromContext(ctx).Error("Windows tunnel companion failed",
		"component", "client", "operation", "windows_tunnel_companion", "stage", stage,
		"reason", reason, "context_state", cancellation, "duration_ms", elapsed.Milliseconds())
}
