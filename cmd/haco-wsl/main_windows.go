//go:build windows && (amd64 || arm64)

// haco-wsl is an internal Windows installation/continuation helper, not another user-facing CLI.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/wslreclaim"
)

func run(ctx context.Context, args []string, stdout, stderr io.Writer, enroll func(context.Context, string) error) int {
	if len(args) != 2 || args[0] != "enroll" {
		fmt.Fprintln(stderr, "Internal Windows helper requires enroll and an exact registration GUID.")
		return 2
	}
	logger, err := logging.NewFromEnv(stderr)
	if err != nil {
		fmt.Fprintln(stderr, "Invalid logging configuration.")
		return 2
	}
	if err := enroll(ctx, args[1]); err != nil {
		logger.Error("Windows installation enrollment failed", "component", "host", "operation", "enroll_wsl", "error", err)
		return 1
	}
	fmt.Fprintln(stdout, "Managed WSL enrollment complete.")
	return 0
}

type helperActions struct {
	enroll func(context.Context, string) error
	launch func(context.Context, string, string) (int, error)
	worker func(context.Context, string, string) error
	status func(context.Context, string, string) (wslreclaim.PreparedStatus, error)
}

func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer, actions helperActions) int {
	if len(args) == 0 || args[0] == "enroll" {
		return run(ctx, args, stdout, stderr, actions.enroll)
	}
	if len(args) != 3 || (args[0] != "_launch" && args[0] != "_continue" && args[0] != "_status") {
		fmt.Fprintln(stderr, "Invalid internal Windows helper arguments.")
		return 2
	}
	logger, err := logging.NewFromEnv(stderr)
	if err != nil {
		fmt.Fprintln(stderr, "Invalid logging configuration.")
		return 2
	}
	if args[0] == "_status" {
		status, readErr := actions.status(ctx, args[1], args[2])
		if readErr != nil {
			logger.Error("Windows continuation status failed", "component", "host", "operation", "inspect_wsl_worker", "error", readErr)
			return 1
		}
		if err := json.NewEncoder(stdout).Encode(status); err != nil {
			logger.Error("Windows continuation status output failed", "component", "host", "operation", "inspect_wsl_worker", "error", err)
			return 1
		}
		return 0
	}
	operation := "continue_wsl"
	var pid int
	if args[0] == "_launch" {
		operation = "launch_wsl_worker"
		pid, err = actions.launch(ctx, args[1], args[2])
	} else {
		err = actions.worker(ctx, args[1], args[2])
	}
	if err != nil {
		logger.Error("Windows continuation failed", "component", "host", "operation", operation, "error", err)
		return 1
	}
	if args[0] == "_launch" {
		fmt.Fprintf(stdout, "Dispatched Windows worker %d; inspect the prepared operation for completion.\n", pid)
	}
	// Worker stdout is its startup channel and is closed before WSL shutdown.
	// Completion is available only through the persisted operation status.
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	code := dispatch(ctx, os.Args[1:], os.Stdout, os.Stderr, helperActions{wslreclaim.EnrollInstallation, wslreclaim.LaunchPreparedWorker, wslreclaim.ExecutePreparedWorker, wslreclaim.ReadPreparedStatus})
	cancel()
	stop()
	os.Exit(code)
}
