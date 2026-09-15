//go:build linux

package incus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Exercise the real subprocess and pipe/PTY boundary without managing a native
// Incus instance. The child records its actual argv before producing any output.
func shellBoundaryChild(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("HACO_TEST_SHELL_ARGS", filepath.Join(dir, "args"))
	t.Setenv("HACO_TEST_SHELL_SIZE", filepath.Join(dir, "size"))
	script := "#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$HACO_TEST_SHELL_ARGS\"\n" + body
	if err := os.WriteFile(filepath.Join(dir, "incus"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func shellBoundaryArgs(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

func TestShellStreamRejectsInvalidInstanceBeforeStartingProcess(t *testing.T) {
	for _, ref := range []string{"", "--project", "other-project:guest", "../haco-demo", "unmanaged", "haco-demo\n", "haco-demo/other"} {
		t.Run(fmt.Sprintf("%q", ref), func(t *testing.T) {
			dir := shellBoundaryChild(t, "exit 0\n")
			r := New(&fakeRunner{})
			err := r.ShellEnvironmentStream(context.Background(), ref, strings.NewReader(""), io.Discard, io.Discard)
			if !errors.Is(err, core.ErrInvalidArgument) {
				t.Errorf("invalid instance was not refused: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "args")); !os.IsNotExist(err) {
				t.Fatal("invalid instance reached the child process")
			}
		})
	}
}

func TestShellStreamsPreserveTerminalInputOutputAndExit(t *testing.T) {
	for _, target := range []string{"environment", "trusted-host"} {
		for _, sized := range []bool{false, true} {
			for _, exitCode := range []int{0, 23} {
				t.Run(fmt.Sprintf("%s/sized=%t/exit=%d", target, sized, exitCode), func(t *testing.T) {
					body := fmt.Sprintf("if [ -t 0 ]; then /bin/stty size > \"$HACO_TEST_SHELL_SIZE\"; fi\nIFS= read -r line\nprintf 'output:%%s' \"$line\"\nprintf 'separate diagnostic' >&2\nexit %d\n", exitCode)
					dir := shellBoundaryChild(t, body)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					metadata := core.TerminalMetadata{Term: "xterm-256color", ColorTerm: "truecolor", DisplayLanguage: "ja"}
					if sized {
						metadata.Columns, metadata.Rows = 83, 29
					}
					ctx = core.WithTerminalMetadata(ctx, metadata)
					input, writer := io.Pipe()
					defer func() { _ = input.Close() }()
					defer func() { _ = writer.Close() }()
					var stdout, stderr bytes.Buffer
					r := New(trustedHostRunner("RUNNING", "trusted-host", nil))
					run := func() error { return r.ShellEnvironmentStream(ctx, "haco-demo", input, &stdout, &stderr) }
					if target == "trusted-host" {
						prepared, err := r.PrepareTrustedHostShellStream(ctx)
						if err != nil {
							t.Fatal(err)
						}
						// Preparation owns terminal metadata; the later I/O context
						// cannot silently replace it with different dimensions/language.
						run = func() error {
							return prepared(core.WithTerminalMetadata(ctx, core.TerminalMetadata{}), input, &stdout, &stderr)
						}
					}
					done := make(chan error, 1)
					go func() { done <- run() }()
					sent := make(chan error, 1)
					go func() { _, err := io.WriteString(writer, "literal $HOME; --project other\n"); sent <- err }()
					select {
					case err := <-done:
						var exit *exec.ExitError
						if exitCode == 0 && err != nil || exitCode != 0 && (!errors.As(err, &exit) || exit.ExitCode() != exitCode) {
							t.Fatalf("shell exit = %v, want %d", err, exitCode)
						}
					case <-ctx.Done():
						t.Fatal("shell completion waited for client input EOF")
					}
					if err := <-sent; err != nil {
						t.Fatal(err)
					}
					if stdout.String() != "output:literal $HOME; --project other" || stderr.String() != "separate diagnostic" {
						t.Fatalf("shell streams changed: stdout=%q stderr=%q", stdout.String(), stderr.String())
					}
					args := shellBoundaryArgs(t, dir)
					ref, prompt := "haco-demo", environmentPrompt("haco-demo")
					if target == "trusted-host" {
						ref, prompt = "haco-host", trustedHostPrompt
					}
					want := []string{"exec", ref, "--project", "hacocoon", "--force-interactive", "--", "/usr/bin/env", "HACO_SHELL_CONTEXT=" + target, "HACO_PS1=" + prompt, "PROMPT_COMMAND=PS1=$HACO_PS1", "TERM=xterm-256color", "COLORTERM=truecolor"}
					if target == "trusted-host" {
						want = append(want, "HACO_UI_LANGUAGE=ja", "/bin/bash", "-l")
					} else {
						want = append(want, "/bin/bash")
					}
					if !reflect.DeepEqual(args, want) {
						t.Fatalf("shell argv = %q, want %q", args, want)
					}
					if sized {
						data, err := os.ReadFile(filepath.Join(dir, "size"))
						if err != nil || string(data) != "29 83\n" {
							t.Fatalf("child terminal size = %q, %v", data, err)
						}
					}
				})
			}
		}
	}
}

func TestShellStreamPreparationAndInvalidIOCannotStartProcess(t *testing.T) {
	dir := shellBoundaryChild(t, "exit 0\n")
	var absent *Runtime
	if _, err := absent.PrepareTrustedHostShellStream(context.Background()); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("nil runtime accepted: %v", err)
	}
	r := New(trustedHostRunner("RUNNING", "unowned", nil))
	if prepared, err := r.PrepareTrustedHostShellStream(context.Background()); err == nil || prepared != nil {
		t.Fatal("unowned Host returned a prepared shell")
	}
	r = New(trustedHostRunner("RUNNING", "trusted-host", nil))
	prepared, err := r.PrepareTrustedHostShellStream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, streams := range []struct {
		input    io.Reader
		out, err io.Writer
	}{{nil, io.Discard, io.Discard}, {strings.NewReader(""), nil, io.Discard}, {strings.NewReader(""), io.Discard, nil}} {
		if err := prepared(context.Background(), streams.input, streams.out, streams.err); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("invalid Host shell streams accepted: %v", err)
		}
		if err := r.ShellEnvironmentStream(context.Background(), "haco-demo", streams.input, streams.out, streams.err); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("invalid Environment shell streams accepted: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "args")); !os.IsNotExist(err) {
		t.Fatal("failed preparation or invalid streams launched a child")
	}
}

type shellOutputFailure struct{ err error }

func (w shellOutputFailure) Write([]byte) (int, error) { return 0, w.err }

func TestShellStreamPropagatesOutputFailureAndCancellation(t *testing.T) {
	t.Run("output failure", func(t *testing.T) {
		shellBoundaryChild(t, "printf 'final output'\n")
		failure := errors.New("output consumer closed")
		err := New(&fakeRunner{}).ShellEnvironmentStream(context.Background(), "haco-demo", strings.NewReader(""), shellOutputFailure{failure}, io.Discard)
		if !errors.Is(err, failure) {
			t.Fatalf("lost output failure: %v", err)
		}
	})
	t.Run("cancel running child", func(t *testing.T) {
		shellBoundaryChild(t, "printf 'shell ready'\nIFS= read -r line\n")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		input, writer := io.Pipe()
		defer func() { _ = input.Close() }()
		defer func() { _ = writer.Close() }()
		out := &terminalTranscript{changed: make(chan struct{}, 1)}
		done := make(chan error, 1)
		go func() { done <- New(&fakeRunner{}).ShellEnvironmentStream(ctx, "haco-demo", input, out, io.Discard) }()
		out.wait(t, ctx, "shell ready")
		cancel()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("canceled shell returned success")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("canceled child did not terminate")
		}
	})
}

func TestDirectEnvironmentShellPreservesSeparateStreamsAndExit(t *testing.T) {
	dir := shellBoundaryChild(t, "/bin/cat\nprintf 'diagnostic' >&2\nexit 19\n")
	r := New(&fakeRunner{})
	var stdout, stderr bytes.Buffer
	r.stdin, r.stdout, r.stderr = strings.NewReader("literal input"), &stdout, &stderr
	err := r.ShellEnvironment(context.Background(), "haco-demo")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 19 || stdout.String() != "literal input" || stderr.String() != "diagnostic" {
		t.Fatalf("lost direct shell result: %q / %q / %v", stdout.String(), stderr.String(), err)
	}
	if want := []string{"exec", "haco-demo", "--project", "hacocoon", "--", "/bin/bash"}; !reflect.DeepEqual(shellBoundaryArgs(t, dir), want) {
		t.Fatal("direct shell changed instance, project or command")
	}
}
