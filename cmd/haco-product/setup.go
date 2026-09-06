package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
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
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: haco setup\nPrepare the installed Host through its controller. Run haco doctor afterward.")
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(stderr, "haco: usage: haco setup")
		return 2
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
	if err := client.SetupHost(ctx); err != nil {
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
