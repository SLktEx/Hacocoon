package main

import (
	"context"
	"encoding/json"
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

func runDoctor(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	return doctor(ctx, args, os.Stdout, os.Stderr)
}

func doctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: haco doctor [--json]")
		return 0
	}
	jsonOutput := len(args) == 1 && args[0] == "--json"
	if len(args) != 0 && !jsonOutput {
		fmt.Fprintln(stderr, "haco: usage: haco doctor [--json]")
		return 2
	}
	logger, err := logging.NewFromEnv(stderr)
	if err != nil {
		fmt.Fprintln(stderr, "haco: invalid logging configuration")
		return 1
	}
	logging.SetRoot(logger)
	fail := func(message string) int {
		logging.Root().ErrorContext(ctx, message, "component", "cli", "operation", "doctor")
		return 1
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		return fail("Cannot open the Physical Host controller client")
	}
	response, err := client.Doctor(ctx)
	if err != nil {
		switch {
		case ctx.Err() != nil:
			return fail("Host diagnostics timed out or were canceled")
		case errors.Is(err, control.ErrUnavailable):
			return fail("Physical Host controller is unavailable; check the WSL installation and controller service")
		case errors.Is(err, control.ErrProtocol):
			return fail("Physical Host controller returned invalid or incompatible diagnostics")
		default:
			return fail("Physical Host controller could not provide diagnostics; check the current installation")
		}
	}
	if jsonOutput {
		err = json.NewEncoder(stdout).Encode(response)
	} else {
		_, err = fmt.Fprintf(stdout, "Hacocoon Host diagnostics\ncontroller: %q (commit %q, protocol %d)\n", response.Controller.Version, response.Controller.Commit, response.ProtocolVersion)
		for _, check := range response.Checks {
			if err != nil {
				break
			}
			_, err = fmt.Fprintf(stdout, "%s: %s - %s\n", check.Name, check.Status, check.Summary)
		}
	}
	if err != nil {
		return fail("Could not write Host diagnostics")
	}
	if !response.Healthy() {
		return fail("Host diagnostic checks did not pass; see the reported checks")
	}
	return 0
}
