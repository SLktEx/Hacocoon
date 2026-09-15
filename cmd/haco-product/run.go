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
)

func runTemporary(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return temporaryCommand(ctx, args, os.Stdout, os.Stderr)
}

func temporaryCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco run", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	workspace := flags.String("workspace", "", cliMessage("run.flag_workspace"))
	base := flags.String("base", "", cliMessage("detail.base"))
	noOCI := flags.Bool("no-oci", false, cliMessage("flag.no_oci"))
	readOnly := flags.Bool("read-only", false, cliMessage("run.flag_readonly"))
	remove := flags.Bool("rm", true, cliMessage("run.flag_rm"))
	asJSON := flags.Bool("json", false, cliMessage("flag.json"))
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
	mode := core.WorkspaceReadWrite
	if *readOnly {
		mode = core.WorkspaceReadOnly
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot connect to the controller")
		return 1
	}
	result, runErr := client.Run(ctx, runapp.Spec{
		WorkspacePath: *workspace, Base: core.BaseName(*base), SkipDefaultResource: *noOCI,
		AccessMode: mode, Argv: flags.Args(),
	})
	if ctx.Err() != nil {
		fmt.Fprintln(diagnostic, "haco: execution canceled; controller cleanup was requested but is not confirmed here. Inspect haco env list.")
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
			fmt.Fprintln(diagnostic, "haco: command output was truncated")
		}
	}
	if !result.CleanedUp {
		if result.Environment != "" {
			fmt.Fprintf(diagnostic, "haco: execution or cleanup failed; inspect Environment %s. Cleanup is not confirmed.\n", displayCell(result.Environment))
		} else {
			fmt.Fprintln(diagnostic, "haco: temporary execution failed before a result was received")
		}
		return 1
	}
	code := result.Execution.ExitCode
	if code < 0 || code > 255 {
		fmt.Fprintln(diagnostic, "haco: invalid command exit status")
		return 1
	}
	if runErr != nil {
		var exit interface{ ExitCode() int }
		if !errors.As(runErr, &exit) || exit.ExitCode() != code || code == 0 {
			fmt.Fprintln(diagnostic, "haco: execution failed; Environment cleanup completed")
			return 1
		}
	}
	return code
}
