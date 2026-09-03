package host

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

const DefaultCaptureLimit = 4 << 20

const truncationMarkerPrefix = "\n[haco: output truncated; total-bytes="
const truncationMarkerSuffix = "]\n"

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

// CommandError preserves the underlying process error while making ordinary
// user-facing failures actionable. The rendered command and captured streams
// are redacted with the same rules as structured logging before they leave the
// runner, so higher-level wrapping keeps useful diagnostics without printing
// obvious credential-bearing arguments or output verbatim.
type CommandError struct {
	command  string
	exitCode int
	stdout   string
	stderr   string
	cause    error
}

func (e *CommandError) Error() string {
	if e == nil {
		return "host command failed"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "host command failed: %s\nexit code: %d", e.command, e.exitCode)
	if e.cause != nil {
		fmt.Fprintf(&out, "\nerror: %s", logging.RedactString(e.cause.Error()))
	}
	if strings.TrimSpace(e.stdout) != "" {
		fmt.Fprintf(&out, "\nstdout:\n%s", strings.TrimRight(e.stdout, "\r\n"))
	}
	if strings.TrimSpace(e.stderr) != "" {
		fmt.Fprintf(&out, "\nstderr:\n%s", strings.TrimRight(e.stderr, "\r\n"))
	}
	return out.String()
}

func (e *CommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

type ExecRunner struct {
	// MaxOutputBytes is the maximum number of child-output bytes retained
	// independently for stdout and stderr. Zero uses DefaultCaptureLimit. The
	// child continues to run after the limit is reached; excess output is
	// discarded rather than back-pressuring or terminating the process.
	MaxOutputBytes int
}

func (r ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	started := time.Now()
	logger := logging.FromContext(ctx).With(
		"component", commandComponent(name, args),
		"command", filepath.Base(name),
	)
	logger.DebugContext(ctx, "executing host command", "args", logging.SanitizeArgs(args))

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
		Stdout:          stdout.Output(),
		Stderr:          stderr.Output(),
		ExitCode:        0,
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
		StdoutBytes:     stdout.TotalBytes(),
		StderrBytes:     stderr.TotalBytes(),
	}
	if err == nil {
		logger.DebugContext(ctx, "host command completed",
			"duration_ms", time.Since(started).Milliseconds(),
			"exit_code", 0,
		)
		return result, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.ExitCode = exit.ExitCode()
		logger.DebugContext(ctx, "host command failed",
			"duration_ms", time.Since(started).Milliseconds(),
			"exit_code", result.ExitCode,
			"error", err,
		)
		return result, newCommandError(name, args, result, err)
	}
	result.ExitCode = -1
	logger.DebugContext(ctx, "host command failed",
		"duration_ms", time.Since(started).Milliseconds(),
		"exit_code", result.ExitCode,
		"error", err,
	)
	return result, newCommandError(name, args, result, err)
}

func newCommandError(name string, args []string, result Result, cause error) error {
	return &CommandError{
		command:  formatCommand(name, args),
		exitCode: result.ExitCode,
		stdout:   logging.RedactString(result.Stdout),
		stderr:   logging.RedactString(result.Stderr),
		cause:    cause,
	}
}

func formatCommand(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, formatCommandArg(logging.RedactString(name)))
	for _, arg := range logging.SanitizeArgs(args) {
		parts = append(parts, formatCommandArg(arg))
	}
	return strings.Join(parts, " ")
}

func formatCommandArg(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\r\n\"'") {
		return arg
	}
	return strconv.Quote(arg)
}

func commandComponent(name string, args []string) string {
	switch strings.ToLower(filepath.Base(name)) {
	case "incus", "lxc":
		if len(args) > 0 {
			switch args[0] {
			case "network":
				return "network"
			case "storage":
				return "storage"
			}
		}
		return "incus"
	case "git":
		return "git"
	case "nerdctl", "docker", "containerd", "ctr":
		return "oci"
	case "btrfs", "mount", "umount", "losetup", "truncate", "fallocate":
		return "storage"
	case "ip", "iptables", "ip6tables", "nft":
		return "network"
	default:
		return "host"
	}
}

// DecodeCapturedOutput removes the trusted runner's truncation marker and
// returns the total number of bytes observed on that stream. Callers that only
// need to display output may keep the marker; machine-oriented callers can use
// this helper to expose structured truncation metadata.
func DecodeCapturedOutput(output string) (clean string, truncated bool, totalBytes int64) {
	markerStart := strings.LastIndex(output, truncationMarkerPrefix)
	if markerStart < 0 || !strings.HasSuffix(output, truncationMarkerSuffix) {
		return output, false, int64(len(output))
	}
	rawTotal := strings.TrimSuffix(output[markerStart+len(truncationMarkerPrefix):], truncationMarkerSuffix)
	total, err := strconv.ParseInt(rawTotal, 10, 64)
	if err != nil || total <= int64(markerStart) {
		return output, false, int64(len(output))
	}
	return output[:markerStart], true, total
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

func (b *boundedBuffer) Output() string {
	output := string(b.buf)
	if !b.truncated {
		return output
	}
	return output + fmt.Sprintf("%s%d%s", truncationMarkerPrefix, b.total, truncationMarkerSuffix)
}

func (b *boundedBuffer) Truncated() bool   { return b.truncated }
func (b *boundedBuffer) TotalBytes() int64 { return b.total }
