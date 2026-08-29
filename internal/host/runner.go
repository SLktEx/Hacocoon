package host

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

const maxCapturedOutputBytes = 16 << 20

var ErrOutputLimitExceeded = errors.New("command output exceeded capture limit")

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(context.Context, string, ...string) (Result, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return runWithCaptureLimit(ctx, maxCapturedOutputBytes, name, args...)
}

func runWithCaptureLimit(ctx context.Context, limit int, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout := newLimitedBuffer(limit)
	stderr := newLimitedBuffer(limit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: 0}
	var captureErr error
	if stdout.Exceeded() || stderr.Exceeded() {
		captureErr = ErrOutputLimitExceeded
	}
	if err == nil {
		return result, captureErr
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.ExitCode = exit.ExitCode()
		return result, errors.Join(err, captureErr)
	}
	result.ExitCode = -1
	return result, errors.Join(err, captureErr)
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	if limit < 0 {
		limit = 0
	}
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		if original > 0 {
			b.exceeded = true
		}
		return original, nil
	}
	if len(p) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.exceeded = true
		return original, nil
	}
	_, _ = b.buffer.Write(p)
	return original, nil
}

func (b *limitedBuffer) String() string { return b.buffer.String() }
func (b *limitedBuffer) Exceeded() bool { return b.exceeded }
