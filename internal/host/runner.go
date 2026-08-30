package host

import (
	"context"
	"errors"
	"os/exec"
)

const DefaultCaptureLimit = 4 << 20

type Result struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	StdoutTruncated bool
	StderrTruncated bool
	StdoutBytes     int64
	StderrBytes     int64
}

type Runner interface {
	Run(context.Context, string, ...string) (Result, error)
}

type ExecRunner struct {
	// MaxOutputBytes is the maximum number of bytes retained independently for
	// stdout and stderr. Zero uses DefaultCaptureLimit. The child continues to
	// run after the limit is reached; excess output is discarded rather than
	// back-pressuring or terminating the process.
	MaxOutputBytes int
}

func (r ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	limit := r.MaxOutputBytes
	if limit <= 0 {
		limit = DefaultCaptureLimit
	}

	cmd := exec.CommandContext(ctx, name, args...)
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
		StdoutBytes:     stdout.TotalBytes(),
		StderrBytes:     stderr.TotalBytes(),
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
	buf       []byte
	limit     int
	total     int64
	truncated bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	if limit < 0 {
		limit = 0
	}
	return &boundedBuffer{buf: make([]byte, 0, min(limit, 64*1024)), limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.total += int64(len(p))
	remaining := b.limit - len(b.buf)
	if remaining > 0 {
		keep := len(p)
		if keep > remaining {
			keep = remaining
		}
		b.buf = append(b.buf, p[:keep]...)
	}
	if b.total > int64(b.limit) {
		b.truncated = true
	}
	// Always report the full write so noisy untrusted children cannot turn the
	// memory bound into a broken-pipe/control-flow change.
	return len(p), nil
}

func (b *boundedBuffer) String() string    { return string(b.buf) }
func (b *boundedBuffer) Truncated() bool   { return b.truncated }
func (b *boundedBuffer) TotalBytes() int64 { return b.total }
