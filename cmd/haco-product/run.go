package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	"golang.org/x/term"
)

func runTemporary(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return temporaryCommand(ctx, args, os.Stdout, os.Stderr)
}

func temporaryCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	return temporaryCommandWithInput(ctx, args, os.Stdin, out, diagnostic)
}

func temporaryCommandWithInput(ctx context.Context, args []string, stdin io.Reader, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco run", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	workspace := flags.String("workspace", "", cliMessage("run.flag_workspace"))
	base := flags.String("base", "", cliMessage("flag.base"))
	noOCI := flags.Bool("no-oci", false, cliMessage("flag.no_oci"))
	readOnly := flags.Bool("read-only", false, cliMessage("run.flag_readonly"))
	remove := flags.Bool("rm", true, cliMessage("run.flag_rm"))
	asJSON := flags.Bool("json", false, cliMessage("flag.json"))
	var interactive, tty bool
	flags.BoolVar(&interactive, "interactive", false, cliMessage("run.flag_input"))
	flags.BoolVar(&interactive, "i", false, cliMessage("run.flag_input"))
	flags.BoolVar(&tty, "tty", false, cliMessage("run.flag_tty"))
	flags.BoolVar(&tty, "t", false, cliMessage("run.flag_tty"))
	flags.BoolVar(&tty, "it", false, cliMessage("run.flag_tty"))
	flags.Usage = func() {
		commandHelp(diagnostic, "run", cliLanguage())
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() == 0 || !*remove || (*readOnly && *workspace == "") {
		flags.Usage()
		return 2
	}
	if tty {
		interactive = true
	}
	if interactive && *asJSON {
		fmt.Fprintln(diagnostic, cliMessage("run.stream_json"))
		return 2
	}
	if tty {
		input, ok := stdin.(interface{ Fd() uintptr })
		if !ok || !term.IsTerminal(int(input.Fd())) {
			fmt.Fprintln(diagnostic, cliMessage("run.tty_required"))
			return 2
		}
	}
	mode := core.WorkspaceReadWrite
	if *readOnly {
		mode = core.WorkspaceReadOnly
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.controller"))
		return 1
	}
	spec := runapp.Spec{
		WorkspacePath: *workspace, Base: core.BaseName(*base), SkipDefaultResource: *noOCI,
		AccessMode: mode, Argv: flags.Args(),
	}
	var result runapp.Result
	var runErr error
	if interactive {
		result, runErr = client.RunStream(ctx, spec, tty, stdin, out, diagnostic)
	} else {
		result, runErr = client.Run(ctx, spec)
	}
	if ctx.Err() != nil {
		fmt.Fprintln(diagnostic, cliMessage("run.canceled"))
		return 130
	}
	if *asJSON {
		if err := json.NewEncoder(out).Encode(result); err != nil {
			return 1
		}
	} else {
		if _, err := io.WriteString(out, result.Execution.Stdout); err != nil {
			return 1
		}
		if _, err := io.WriteString(diagnostic, result.Execution.Stderr); err != nil {
			return 1
		}
		if result.Execution.StdoutTruncated || result.Execution.StderrTruncated {
			fmt.Fprintln(diagnostic, cliMessage("run.truncated"))
		}
	}
	if !result.CleanedUp {
		if result.Environment != "" {
			fmt.Fprintln(diagnostic, cliMessage("run.cleanup_unknown", displayCell(result.Environment)))
		} else {
			fmt.Fprintln(diagnostic, cliMessage("run.no_result"))
		}
		return 1
	}
	code := result.Execution.ExitCode
	if code < 0 || code > 255 {
		fmt.Fprintln(diagnostic, cliMessage("run.invalid_exit"))
		return 1
	}
	if runErr != nil {
		var exit interface{ ExitCode() int }
		if !errors.As(runErr, &exit) || exit.ExitCode() != code || code == 0 {
			fmt.Fprintln(diagnostic, cliMessage("run.execution_failed"))
			return 1
		}
	}
	return code
}
