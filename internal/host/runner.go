package host

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

const DefaultOutputLimit = 4 << 20 // 4 MiB per stream

type Result struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	StdoutTruncated bool
	StderrTruncated bool
}

type Runner interface {
	Run(context.Context, string, ...string) (Result, error)
}

type ExecRunner struct {
	// OutputLimit bounds each captured stdout/stderr stream. Zero uses DefaultOutputLimit.
	OutputLimit int
}

func (r ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	limit := r.OutputLimit
	if limit <= 0 {
		limit = DefaultOutputLimit
	}
	stdout := newBoundedBuffer(limit)
	stderr := newBoundedBuffer(limit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	result := Result{
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		ExitCode:        0,
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
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

type boundedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newBoundedBuffer(limit int) *boundedBuffer { return &boundedBuffer{limit: limit} }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		if original > 0 {
			b.truncated = true
		}
		return original, nil
	}
	if len(p) > remaining {
		b.truncated = true
		p = p[:remaining]
	}
	_, _ = b.buf.Write(p)
	return original, nil
}

func (b *boundedBuffer) String() string  { return b.buf.String() }
func (b *boundedBuffer) Truncated() bool { return b.truncated }
