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

func writeDoctorUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: haco doctor [--json] [environment]")
	fmt.Fprintln(out, "       haco doctor --fix [--json] <environment>")
}

func parseDoctorArgs(args []string) (jsonOutput, fix bool, target string, ok bool) {
	for _, arg := range args {
		switch arg {
		case "--json":
			if jsonOutput {
				return false, false, "", false
			}
			jsonOutput = true
		case "--fix":
			if fix {
				return false, false, "", false
			}
			fix = true
		default:
			if arg == "" || strings.HasPrefix(arg, "-") || target != "" {
				return false, false, "", false
			}
			target = arg
		}
	}
	if fix && target == "" {
		return false, false, "", false
	}
	return jsonOutput, fix, target, true
}

func doctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		writeDoctorUsage(stdout)
		return 0
	}
	jsonOutput, fix, target, ok := parseDoctorArgs(args)
	if !ok {
		fmt.Fprint(stderr, "haco: ")
		writeDoctorUsage(stderr)
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
	if target != "" {
		report, err := diagnoseEnvironmentWithGit(ctx, client, target)
		if err != nil {
			return fail("Could not inspect Environment; check haco env list and controller availability")
		}
		if fix {
			report, err = repairEnvironmentGitBroker(ctx, client, target, report)
			if err != nil {
				return fail("Could not repair the managed Git broker; inspect local broker wiring and retry")
			}
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
		_, err = fmt.Fprintf(stdout, "Hacocoon Host diagnostics\ncontroller: %q (commit %q, protocol %d)\n", response.Controller.Version, response.Controller.Commit, response.ProtocolVersion)
		for _, check := range response.Checks {
			if err != nil {
				break
			}
			_, err = fmt.Fprintf(stdout, "%s: %s - %s\n", check.Name, check.Status, check.Summary)
			if err == nil && check.Action != "" {
				_, err = fmt.Fprintf(stdout, "  Next: %s\n", check.Action)
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
