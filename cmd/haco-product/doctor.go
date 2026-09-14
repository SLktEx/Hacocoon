package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func runDoctor(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, controllerStartupTimeout+45*time.Second)
	defer cancel()
	return doctor(ctx, args, os.Stdout, os.Stderr)
}

func doctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, cliMessage("usage", "haco doctor [--json] [environment]"))
		return 0
	}
	jsonOutput := false
	if len(args) > 0 && args[0] == "--json" {
		jsonOutput = true
		args = args[1:]
	}
	if len(args) > 1 || (len(args) == 1 && (args[0] == "" || strings.HasPrefix(args[0], "-"))) {
		fmt.Fprintln(stderr, cliMessage("error.usage", "haco doctor [--json] [environment]"))
		return 2
	}
	target := ""
	if len(args) == 1 {
		target = args[0]
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
	if target != "" {
		report, err := diagnoseEnvironment(ctx, client, target)
		if err != nil {
			return fail("Could not inspect Environment; check haco env list and controller availability")
		}
		return writeEnvironmentDoctor(stdout, report, jsonOutput)
	}
	response, err := collectDoctor(ctx, client)
	if err != nil {
		switch {
		case ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded):
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
		_, err = fmt.Fprint(stdout, cliMessage("doctor.header", response.Controller.Version, response.Controller.Commit, response.ProtocolVersion))
		for _, check := range response.Checks {
			if err != nil {
				break
			}
			_, err = fmt.Fprintf(stdout, "%s: %s - %s\n", check.Name, check.Status, check.Summary)
			if err == nil && check.Action != "" {
				_, err = fmt.Fprint(stdout, cliMessage("doctor.next", check.Action))
			}
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

type hostDoctorClient interface {
	Ping(context.Context) (controlapi.PingResponse, error)
	Doctor(context.Context) (controlapi.DoctorResponse, error)
}

// A normal WSL --exec invocation may run before systemd binds the enabled
// controller socket. Wait only through read-only ping, then diagnose once.
// Failed checks and protocol rejection are never retried or repaired.
func collectDoctor(ctx context.Context, client hostDoctorClient) (controlapi.DoctorResponse, error) {
	readyCtx, cancel := context.WithTimeout(ctx, controllerStartupTimeout)
	err := waitForController(readyCtx, func(ctx context.Context) error {
		_, err := client.Ping(ctx)
		return err
	})
	cancel()
	if err != nil {
		return controlapi.DoctorResponse{}, err
	}
	return client.Doctor(ctx)
}
