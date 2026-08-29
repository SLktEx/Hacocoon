package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const maxCommandOutputBytes = 16 << 20

var ErrOutputLimit = errors.New("command output limit exceeded")

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(context.Context, string, ...string) (Result, error)
}

type ExecRunner struct{}

type cappedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		_, _ = b.buffer.Write(p[:remaining])
	}
	if original > remaining {
		b.exceeded = true
		b.cancel()
	}
	return original, nil
}

func (b *cappedBuffer) String() string { return b.buffer.String() }

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	commandCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, name, args...)
	cmd.WaitDelay = 2 * time.Second
	stdout := cappedBuffer{limit: maxCommandOutputBytes, cancel: cancel}
	stderr := cappedBuffer{limit: maxCommandOutputBytes, cancel: cancel}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: 0}
	if stdout.exceeded || stderr.exceeded {
		result.ExitCode = -1
		return result, fmt.Errorf("%s produced more than %d bytes on stdout or stderr: %w", name, maxCommandOutputBytes, ErrOutputLimit)
	}
	if err == nil {
		return result, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.ExitCode = exit.ExitCode()
		return result, err
	}
	result.ExitCode = -1
	return result, err
}
