package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type execFailureClient struct{ err error }

func (f execFailureClient) ExecStream(context.Context, string, core.ProcessRequest, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, f.err
}

type execCommandExit int

func (e execCommandExit) Error() string { return "command failed" }
func (e execCommandExit) ExitCode() int { return int(e) }

func TestExecFlagsDoNotConsumeGuestOptionsAndTerminalInputIsExplicit(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		input string
		tty   bool
		work  string
	}{
		{[]string{"-it", "-w", "/project", "dev", "bash", "-l"}, "private", true, "/project"},
		{[]string{"-ti", "dev", "--", "bash", "-l"}, "private", true, "/workspace"},
		{[]string{"-t", "dev", "--", "bash", "-l"}, "", true, "/workspace"},
	} {
		f := &execCLIStub{}
		in := closedTerminalInput{Reader: strings.NewReader("private"), fd: 123}
		if code := execCommand(context.Background(), f, tc.args, in, io.Discard, io.Discard); code != 17 || f.input != tc.input || f.request.TTY != tc.tty || f.request.WorkingDirectory != tc.work || len(f.request.Argv) != 2 || f.request.Argv[1] != "-l" {
			t.Fatal(code, f)
		}
	}
}
func TestExecUsageAndErrorsPreserveExitSemantics(t *testing.T) {
	for _, args := range [][]string{{"-w"}, {"--bad"}, {"../dev", "--", "true"}, {"dev"}, {"-w", "relative", "dev", "--", "true"}} {
		f := &execCLIStub{}
		if code := execCommand(context.Background(), f, args, strings.NewReader(""), io.Discard, io.Discard); code != 2 || f.name != "" {
			t.Fatal(args, code, f)
		}
	}
	if code := execCommand(context.Background(), &execCLIStub{}, []string{"--help"}, strings.NewReader(""), io.Discard, io.Discard); code != 0 {
		t.Fatal(code)
	}
	for _, tc := range []struct {
		err  error
		code int
	}{{context.Canceled, 130}, {execCommandExit(43), 43}, {errors.New("stopped Environment"), 1}} {
		if code := execCommand(context.Background(), execFailureClient{tc.err}, []string{"dev", "--", "true"}, strings.NewReader(""), io.Discard, io.Discard); code != tc.code {
			t.Fatal(code, tc)
		}
	}
}
