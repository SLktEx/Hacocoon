package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
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

type execCompletionRaceClient struct {
	started chan struct{}
	finish  chan struct{}
	result  core.ExecutionResult
	err     error
}

func (f *execCompletionRaceClient) ExecStream(context.Context, string, core.ProcessRequest, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error) {
	close(f.started)
	<-f.finish
	return f.result, f.err
}

func TestExecLocalCancellationWinsRacingStreamCompletion(t *testing.T) {
	for _, completion := range []struct {
		name   string
		result core.ExecutionResult
		err    error
		code   int
	}{
		{"transport-EOF", core.ExecutionResult{}, io.ErrUnexpectedEOF, 1},
		{"protocol-error", core.ExecutionResult{}, control.ErrProtocol, 1},
		{"remote-exit", core.ExecutionResult{ExitCode: 23}, execCommandExit(23), 23},
		{"result-exit", core.ExecutionResult{ExitCode: 17}, nil, 17},
		{"success", core.ExecutionResult{}, nil, 0},
	} {
		for _, canceled := range []bool{false, true} {
			name := completion.name
			if canceled {
				name += "/canceled"
			}
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				client := &execCompletionRaceClient{started: make(chan struct{}), finish: make(chan struct{}), result: completion.result, err: completion.err}
				done := make(chan int, 1)
				var diagnostic strings.Builder
				go func() {
					done <- execCommand(ctx, client, []string{"dev", "--", "sleep", "600"}, strings.NewReader(""), io.Discard, &diagnostic)
				}()
				select {
				case <-client.started:
				case <-time.After(time.Second):
					t.Fatal("execution did not start")
				}
				if canceled {
					cancel()
				}
				// The user's cancellation and the transport/result are separate
				// observations. A stream implementation need not return ctx.Err.
				close(client.finish)
				want := completion.code
				if canceled {
					want = 130
				}
				select {
				case code := <-done:
					if code != want {
						t.Fatalf("exit = %d, want %d", code, want)
					}
					if canceled && diagnostic.Len() != 0 {
						t.Fatal("local cancellation emitted a racing transport diagnostic")
					}
				case <-time.After(time.Second):
					t.Fatal("execution did not finish")
				}
			})
		}
	}
}
