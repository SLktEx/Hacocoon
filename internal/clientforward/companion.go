package clientforward

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

// The parent owns a real pipe, not an exec stdin copy goroutine. Parent death
// closes its write handle even when no bytes are being written. Cancellation
// closes the lease first; WaitDelay kills only the exact unresponsive child.
func runCompanion(ctx context.Context, cmd *exec.Cmd, request delegation, out, diagnostic io.Writer) (int, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return 1, err
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = reader, out, diagnostic
	cmd.Cancel = func() error { return writer.Close() }
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Start(); err != nil {
		return 1, err
	}
	_ = reader.Close()
	preparation := time.AfterFunc(10*time.Second, func() { _ = writer.Close(); _ = cmd.Process.Kill() })
	err = writeDelegation(writer, request)
	if !preparation.Stop() || err != nil {
		_ = writer.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return 1, errors.New("windows tunnel preparation failed")
	}
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
