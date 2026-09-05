package incus

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	hacocoonPromptCommand = "PS1=$HACO_PS1"
	trustedHostPrompt      = `\[\e[1;33;41m\][HACO-HOST]\[\e[0m\] \u@\h:\w\$ `
)

// ShellEnvironmentStream opens a shell through Incus while attaching it to
// caller-provided streams. Force interactive mode because controller streams
// are sockets rather than terminal file descriptors, so Incus cannot infer PTY
// mode from the controller process stdio.
func (r *Runtime) ShellEnvironmentStream(ctx context.Context, ref string, stdin io.Reader, stdout, stderr io.Writer) error {
	if r == nil || ref == "" || stdin == nil || stdout == nil || stderr == nil {
		return core.ErrInvalidArgument
	}
	argv := interactiveShellWithPrompt(
		[]string{"/bin/bash"},
		environmentPrompt(ref),
		"environment",
		core.TerminalMetadataFromContext(ctx),
	)
	_, err := r.execInteractiveStream(ctx, ref, argv, stdin, stdout, stderr)
	return err
}

// PrepareTrustedHostShellStream reconciles the trusted logical Host before the
// controller acknowledges an interactive stream. The returned function owns
// only the long-lived shell I/O; Incus authority remains in the controller.
func (r *Runtime) PrepareTrustedHostShellStream(ctx context.Context) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	if r == nil {
		return nil, core.ErrInvalidArgument
	}
	if err := r.EnsureTrustedHost(ctx); err != nil {
		return nil, err
	}
	terminal := core.TerminalMetadataFromContext(ctx)
	return func(runCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
		if stdin == nil || stdout == nil || stderr == nil {
			return core.ErrInvalidArgument
		}
		argv := interactiveShellWithPrompt(
			[]string{"/bin/bash", "-l"},
			trustedHostPrompt,
			"trusted-host",
			terminal,
		)
		_, err := r.execInteractiveStream(runCtx, trustedHostName, argv, stdin, stdout, stderr)
		return err
	}, nil
}

func interactiveShellWithPrompt(argv []string, prompt, shellContext string, terminal core.TerminalMetadata) []string {
	wrapped := []string{
		"/usr/bin/env",
		"HACO_SHELL_CONTEXT=" + shellContext,
		"HACO_PS1=" + prompt,
		"PROMPT_COMMAND=" + hacocoonPromptCommand,
	}
	if terminal.Term != "" {
		wrapped = append(wrapped, "TERM="+terminal.Term)
	}
	if terminal.ColorTerm != "" {
		wrapped = append(wrapped, "COLORTERM="+terminal.ColorTerm)
	}
	return append(wrapped, argv...)
}

func environmentPrompt(ref string) string {
	name := strings.TrimPrefix(ref, "haco-")
	name = safePromptLabel(name)
	return `\[\e[1;30;42m\][HACO-ENV:` + name + `]\[\e[0m\] \u@\h:\w\$ `
}

func safePromptLabel(value string) string {
	if value == "" {
		return "?"
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

func (r *Runtime) execInteractiveStream(ctx context.Context, ref string, argv []string, stdin io.Reader, stdout, stderr io.Writer) (core.ExecResult, error) {
	if len(argv) == 0 {
		return core.ExecResult{}, core.ErrInvalidArgument
	}
	args := append([]string{"exec", ref, "--project", r.project, "--force-interactive", "--"}, argv...)
	cmd := exec.CommandContext(ctx, "incus", args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return core.ExecResult{ExitCode: 0}, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return core.ExecResult{ExitCode: exit.ExitCode()}, err
	}
	return core.ExecResult{ExitCode: -1}, err
}
