package gitadapter

import (
	"context"
	"io"
	"regexp"
	"strings"
)

type progressKey struct{}

// WithProgress carries human diagnostics separately from protocol stdout.
func WithProgress(ctx context.Context, writer io.Writer) context.Context {
	return context.WithValue(ctx, progressKey{}, writer)
}

func ProgressWriter(ctx context.Context) io.Writer {
	if w, ok := ctx.Value(progressKey{}).(io.Writer); ok && w != nil {
		return w
	}
	return io.Discard
}

// Only native transfer counters cross the credential boundary. Remote messages,
// paths, URLs and arbitrary stderr may contain secrets even after URL redaction.
var transferProgress = regexp.MustCompile(`^(Enumerating objects|Counting objects|Compressing objects|Receiving objects|Resolving deltas|Updating files|Checking out files): +[0-9]+(% \([0-9]+/[0-9]+\))?(, [0-9.]+ [KMGT]?i?B( \| [0-9.]+ [KMGT]?i?B/s)?)?(, done\.)?\.?$`)

// SafeGitLine returns a bounded, credential-free terminal diagnostic. Unknown
// peer output is intentionally not displayed or put in an error/log.
func SafeGitLine(line string) string {
	if len(line) > 4096 {
		return ""
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "remote: "))
	if transferProgress.MatchString(line) {
		return line
	}
	if strings.HasPrefix(line, "Cloning into ") {
		return "Cloning into managed repository..."
	}
	for _, diagnostic := range []struct{ match, safe string }{
		{"Authentication failed", "Authentication failed; check trusted Host gh authentication"},
		{"Repository not found", "Repository not found; check the registered URL and Host access"},
		{"does not appear to be a git repository", "Remote is not an accessible Git repository"},
		{"Could not resolve host", "Could not resolve remote host"},
		{"Failed to connect", "Failed to connect to remote host"},
		{"SSL certificate problem", "Remote TLS certificate validation failed"},
		{"couldn't find remote ref", "Requested branch is absent from the remote"},
		{"No space left on device", "No space left on device"},
		{"Permission denied", "Permission denied while accessing Git data or remote"},
		{"early EOF", "Git transfer ended early; check the network and retry"},
		{"RPC failed", "Git transfer RPC failed; check the network and remote service"},
		{"could not read Username", "Host authentication is unavailable; authenticate with gh in trusted Host"},
	} {
		if strings.Contains(line, diagnostic.match) || line == diagnostic.safe {
			return diagnostic.safe
		}
	}
	return ""
}

// GitDiagnostic handles arbitrarily split stderr writes and CR progress records
// without retaining unbounded output or leaking fragments of an overlong line.
type GitDiagnostic struct {
	writer   io.Writer
	line     []byte
	overflow bool
	failure  string
	err      error
}

func NewGitDiagnostic(writer io.Writer) *GitDiagnostic { return &GitDiagnostic{writer: writer} }

func (d *GitDiagnostic) Write(p []byte) (int, error) {
	for _, c := range p {
		if c == '\r' || c == '\n' {
			d.Flush()
		} else if !d.overflow {
			if len(d.line) == 4096 {
				d.line = nil
				d.overflow = true
			} else {
				d.line = append(d.line, c)
			}
		}
	}
	return len(p), d.err
}

func (d *GitDiagnostic) Flush() {
	if !d.overflow {
		line := SafeGitLine(string(d.line))
		if line != "" {
			if !transferProgress.MatchString(line) && !strings.HasPrefix(line, "Cloning into ") {
				d.failure = line
			}
			if d.writer != nil && d.err == nil {
				_, d.err = io.WriteString(d.writer, line+"\n")
			}
		}
	}
	d.line = d.line[:0]
	d.overflow = false
}

func (d *GitDiagnostic) Failure() string {
	if d.failure != "" {
		return d.failure
	}
	return "check the registered URL, selected branch, Host authentication, network and available storage"
}
