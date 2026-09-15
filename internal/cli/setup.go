package cli

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

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
	"github.com/SLktEx/Hacocoon/internal/logging"
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
	configureCLIFlags(flags, stderr)
	scriptPath := flags.String("script", "", cliMessage("detail.setup_script"))
	clear := flags.Bool("clear-script", false, cliMessage("detail.setup_clear"))
	reapply := flags.Bool("reapply-script", false, cliMessage("detail.setup_reapply"))
	resultOnly := flags.Bool("script-result", false, cliMessage("detail.setup_result"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			commandHelp(stdout, "setup", cliLanguage())
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
		_, _ = fmt.Fprintln(stderr, cliMessage("setup.invalid_usage"))
		return 2
	}
	update := recipes.Update{Clear: *clear, Reapply: *reapply, ResultOnly: *resultOnly}
	if *scriptPath != "" {
		data, err := recipes.ReadScript(*scriptPath)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.script_unreadable"))
			return 1
		}

		text := string(data)
		update.Script = &text
		if err := update.Validate(); err != nil {
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.invalid_script_options"))
			return 2
		}
	}
	if err := update.Validate(); err != nil || (flags.NArg() == 1 && (*reapply || *resultOnly)) {
		_, _ = fmt.Fprintln(stderr, cliMessage("setup.select_option"))
		return 2
	}

	logger, err := logging.NewFromEnv(stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, cliMessage("error.logging"))
		return 1
	}
	logging.SetRoot(logger)
	fail := func(message string) int {
		logging.Root().ErrorContext(ctx, message, "component", "cli", "operation", "setup")
		return 1
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, cliMessage("setup.client_failed"))
		return fail("Cannot open the Physical Host controller client; rerun the installer")
	}
	if flags.NArg() == 1 {
		response, err := client.SetupProject(ctx, flags.Arg(0), update)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.project_request_failed"))
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
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.truncated"))
		}
		if response.Failed {
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.project_failed"))
			stage, code := safeProjectSetupFailure(result.FailureStage, response.FailureCode)
			logging.Root().ErrorContext(ctx, "Project setup failed; correct the script or Environment and rerun haco setup", "component", "cli", "operation", "setup", "stage", stage, "error_code", code, "exit_code", result.Execution.ExitCode)
			return 1
		}
		if result.Cleared {
			_, _ = fmt.Fprintln(stdout, cliMessage("setup.project_cleared"))
		} else if result.Applied {
			_, _ = fmt.Fprintln(stdout, cliMessage("setup.project_completed"))
		} else {
			_, _ = fmt.Fprintln(stdout, cliMessage("setup.project_empty"))
		}
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "[running] controller_readiness")
	readyCtx, cancelReady := context.WithTimeout(ctx, controllerStartupTimeout)
	readyErr := waitForSetupController(readyCtx, client)
	cancelReady()
	if readyErr != nil {
		_, _ = fmt.Fprintf(stderr, "[failed] controller_readiness reason=%s\n", hostsetup.Reason(readyErr))
		_, _ = fmt.Fprintln(stderr, cliMessage("setup.not_sent"))
		return 1
	}
	_, _ = fmt.Fprintln(stderr, "[succeeded] controller_readiness")
	requestID := ""
	setupErr := client.SetupHostProgress(ctx, update, func(id string, e hostsetup.Event) {
		if requestID == "" {
			requestID = id
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.request"), id)
		}
		_, _ = fmt.Fprintf(stderr, "[%s] %s", e.State, e.Stage)
		if e.Reason != "" {
			_, _ = fmt.Fprint(stderr, " reason=", e.Reason)
		}
		_, _ = fmt.Fprintln(stderr)
	}, func(result recipes.HostResult) {
		_, _ = fmt.Fprint(stderr, cliMessage("setup.host_result", result.State, result.Digest, result.Execution.ExitCode))
		if *resultOnly {
			_, _ = fmt.Fprint(stdout, result.Execution.Stdout)
			_, _ = fmt.Fprint(stderr, result.Execution.Stderr)
			if result.Execution.StdoutTruncated || result.Execution.StderrTruncated {
				_, _ = fmt.Fprintln(stderr, cliMessage("setup.saved_truncated"))
			}
		} else if result.State == "failed" || result.State == "running" {
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.inspect_result"))
		}
	})
	if err := setupErr; err != nil {
		_, _ = fmt.Fprintln(stderr, cliMessage("setup.unconfirmed"))
		_, _ = fmt.Fprintln(stderr, cliMessage("setup.inspect_before_retry"))
		_, _ = fmt.Fprintln(stderr, cliMessage("daily.journal"))
		if requestID != "" {
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.find_request"), requestID)
		}

		var status *control.StatusError
		switch {
		case ctx.Err() != nil:
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.observation_ended"))
			return 1
		case errors.Is(err, control.ErrUnavailable):
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.unavailable"))
			return 1
		case errors.Is(err, control.ErrProtocol):
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.protocol"))
			return 1
		case errors.As(err, &status) && status.Code == "busy":
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.busy"))
			return fail("[running] setup reason=busy; another setup owns the operation. Wait and inspect its diagnostics")
		case errors.As(err, &status) && status.Code == "customization_failed":
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.customization_failed"))
			return 1
		case errors.As(err, &status) && status.Code == "setup_failed":
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.host_failed"))
			return 1
		default:
			_, _ = fmt.Fprintln(stderr, cliMessage("setup.host_failed"))
			return fail("Host setup failed; inspect haco doctor and the setup request in the journal")
		}
	}
	if *resultOnly {
		return 0
	}
	if *clear {
		_, _ = fmt.Fprintln(stdout, cliMessage("setup.host_cleared"))
		return 0
	}
	if *reapply {
		_, _ = fmt.Fprintln(stdout, cliMessage("setup.host_completed"))
		return 0
	}
	if _, err := fmt.Fprintln(stdout, cliMessage("setup.ready")); err != nil {
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
