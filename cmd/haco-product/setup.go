package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

func runSetup(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 16*time.Minute)
	defer cancel()
	return setup(ctx, args, os.Stdout, os.Stderr)
}

func setup(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("haco setup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	scriptPath := flags.String("script", "", "save and run a UTF-8 bash script in the Host or selected Environment")
	clear := flags.Bool("clear-script", false, "remove the selected saved script without running it")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stdout, "Usage: haco setup [--script <path> | --clear-script] [environment]")
			return 0
		}
		return 2
	}
	scriptSelected := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "script" {
			scriptSelected = true
		}
	})
	if flags.NArg() > 1 || (scriptSelected && (*scriptPath == "" || *clear)) {
		fmt.Fprintln(stderr, "haco: usage: haco setup [--script <path> | --clear-script] [environment]")
		return 2
	}
	update := recipes.Update{Clear: *clear}
	if *scriptPath != "" {
		data, err := recipes.ReadScript(*scriptPath)
		if err != nil {
			fmt.Fprintln(stderr, "haco: cannot read a regular UTF-8 setup script (maximum 1 MiB)")
			return 1
		}

		text := string(data)
		update.Script = &text
		if err := update.Validate(); err != nil {
			fmt.Fprintln(stderr, "haco:", err)
			return 2
		}
	}

	logger, err := logging.NewFromEnv(stderr)
	if err != nil {
		fmt.Fprintln(stderr, "haco: invalid logging configuration")
		return 1
	}
	logging.SetRoot(logger)
	fail := func(message string) int {
		logging.Root().ErrorContext(ctx, message, "component", "cli", "operation", "setup")
		return 1
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		return fail("Cannot open the Physical Host controller client; rerun the installer")
	}
	if flags.NArg() == 1 {
		response, err := client.SetupProject(ctx, flags.Arg(0), update)
		if err != nil {
			return fail("Project setup request failed")
		}
		result := response.Result
		if _, err := io.WriteString(stdout, result.Execution.Stdout); err != nil {
			return 1
		}
		if _, err := io.WriteString(stderr, result.Execution.Stderr); err != nil {
			return 1
		}
		if result.Execution.StdoutTruncated || result.Execution.StderrTruncated {
			fmt.Fprintln(stderr, "haco: setup output was truncated")
		}
		if response.Failed {
			return fail("Project setup failed; correct the script or Environment and rerun haco setup")
		}
		if result.Cleared {
			fmt.Fprintln(stdout, "Saved project setup removed.")
		} else if result.Applied {
			fmt.Fprintln(stdout, "Project setup completed.")
		} else {
			fmt.Fprintln(stdout, "No saved project setup. Use haco setup --script <path> <environment>.")
		}
		return 0
	}
	if err := client.SetupHost(ctx, update); err != nil {
		var status *control.StatusError
		switch {
		case ctx.Err() != nil:
			return fail("Host setup timed out or was canceled; inspect haco doctor before retrying")
		case errors.Is(err, control.ErrUnavailable):
			return fail("Physical Host controller is unavailable; rerun the installer")
		case errors.Is(err, control.ErrProtocol):
			return fail("Physical Host controller protocol is incompatible; rerun the current installer")
		case errors.As(err, &status) && status.Code == "busy":
			return fail("Host setup is already running; wait for it to finish before retrying")
		case errors.As(err, &status) && status.Code == "customization_failed":
			fmt.Fprintln(stderr, "haco: Host prepared, but customization failed; fix your script and rerun haco setup --script <path>, or use --clear-script")
			return 1
		case errors.As(err, &status) && status.Code == "setup_failed":
			fmt.Fprintln(stderr, "haco: Host setup failed; run haco doctor, then rerun the installer")
			return 1
		default:
			return fail("Host setup failed; run haco doctor, then rerun the installer")
		}
	}
	if _, err := fmt.Fprintln(stdout, "Host resources prepared. Run haco doctor to verify readiness."); err != nil {
		return fail("Could not write setup result")
	}
	return 0
}
