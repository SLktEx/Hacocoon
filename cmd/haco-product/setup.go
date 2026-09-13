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
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
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
	scriptPath := flags.String("script", "", cliMessage("detail.setup_script"))
	clear := flags.Bool("clear-script", false, cliMessage("detail.setup_clear"))
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
			stage, code := safeProjectSetupFailure(result.FailureStage, response.FailureCode)
			logging.Root().ErrorContext(ctx, "Project setup failed; correct the script or Environment and rerun haco setup", "component", "cli", "operation", "setup", "stage", stage, "error_code", code, "exit_code", result.Execution.ExitCode)
			return 1
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
	fmt.Fprintln(stderr, "[running] controller_readiness")
	readyCtx, cancelReady := context.WithTimeout(ctx, controllerStartupTimeout)
	readyErr := waitForSetupController(readyCtx, client)
	cancelReady()
	if readyErr != nil {
		fmt.Fprintf(stderr, "[failed] controller_readiness reason=%s\n", hostsetup.Reason(readyErr))
		fmt.Fprintln(stderr, "No setup request sent. Check haco doctor and the controller service on the WSL/Linux Physical Host.")
		return 1
	}
	fmt.Fprintln(stderr, "[succeeded] controller_readiness")
	requestID := ""
	if err := client.SetupHostProgress(ctx, update, func(id string, e hostsetup.Event) {
		if requestID == "" {
			requestID = id
			fmt.Fprintln(stderr, "Setup request:", id)
		}
		fmt.Fprintf(stderr, "[%s] %s", e.State, e.Stage)
		if e.Reason != "" {
			fmt.Fprint(stderr, " reason=", e.Reason)
		}
		fmt.Fprintln(stderr)
	}); err != nil {
		fmt.Fprintln(stderr, "Setup completion is not confirmed. Completed stages are shown above; resources may remain. Current resource state is unknown until inspected.")
		fmt.Fprintln(stderr, "Next: haco doctor. Do not delete resources or blindly replay a saved customization script.")
		fmt.Fprintln(stderr, "Diagnostics (WSL/Linux Physical Host, administrator): journalctl -u haco-controller.service --since '30 minutes ago' --no-pager")
		if requestID != "" {
			fmt.Fprintln(stderr, "Find request_id:", requestID)
		}

		var status *control.StatusError
		switch {
		case ctx.Err() != nil:
			fmt.Fprintln(stderr, "Observation canceled or timed out; controller setup may still be running. Inspect diagnostics before another operation.")
			return 1
		case errors.Is(err, control.ErrUnavailable):
			fmt.Fprintln(stderr, "Controller unavailable; inspect the controller service before another operation.")
			return 1
		case errors.Is(err, control.ErrProtocol):
			fmt.Fprintln(stderr, "Controller progress protocol unavailable or incomplete; inspect diagnostics and installed client/controller versions.")
			return 1
		case errors.As(err, &status) && status.Code == "busy":
			return fail("[running] setup reason=busy; another setup owns the operation. Wait and inspect its diagnostics")
		case errors.As(err, &status) && status.Code == "customization_failed":
			fmt.Fprintln(stderr, "haco: stage=customization reason=failed; Host prepared, but customization failed; fix your script and rerun haco setup --script <path>, or use --clear-script")
			return 1
		case errors.As(err, &status) && status.Code == "setup_failed":
			fmt.Fprintln(stderr, "haco: Host setup failed; inspect haco doctor and the setup request in the journal")
			return 1
		default:
			return fail("Host setup failed; inspect haco doctor and the setup request in the journal")
		}
	}
	if _, err := fmt.Fprintln(stdout, "Host resources prepared. Run haco doctor to verify readiness."); err != nil {
		return fail("Could not write setup result")
	}
	return 0
}

// Diagnostics admit fixed categories only; neither backend errors nor arbitrary
// controller response strings belong in logs.
func safeProjectSetupFailure(stage, code string) (string, string) {
	switch stage {
	case "validate", "lookup", "recipe", "start", "execute", "script":
	default:
		stage = "unknown"
	}
	switch code {
	case "internal", "invalid_argument", "not_found", "already_exists", "unsupported", "unavailable", "denied", "busy", "incompatible_state", "recovery_required":
	default:
		code = "internal"
	}
	return stage, code
}

func waitForSetupController(ctx context.Context, client interface {
	Ping(context.Context) (controlapi.PingResponse, error)
}) error {
	readyCtx, cancel := context.WithTimeout(ctx, controllerStartupTimeout)
	defer cancel()
	return waitForController(readyCtx, func(ctx context.Context) error { _, err := client.Ping(ctx); return err })
}
