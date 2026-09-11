package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

// Internal fixed entry for the Windows continuation; not a daily CLI command.
// The client has no Incus/filesystem authority and never retries a mutation.
func runReclaimLinux(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, controllerStartupTimeout+5*time.Minute)
	defer cancel()
	return reclaimLinuxClient(ctx, args, os.Stdout, os.Stderr)
}
func reclaimLinuxClient(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "Invalid internal reclamation arguments.")
		return 2
	}
	target := reclamation.WSLTarget{RegistrationID: args[0], InstallationID: args[1]}
	if target.Validate() != nil {
		fmt.Fprintln(stderr, "Invalid internal reclamation identity.")
		return 2
	}
	logger, err := logging.NewFromEnv(stderr)
	if err != nil {
		fmt.Fprintln(stderr, "Invalid logging configuration.")
		return 2
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		logger.Error("Cannot open reclamation controller", "component", "cli", "operation", "reclaim_storage")
		return 1
	}
	readyCtx, cancel := context.WithTimeout(ctx, controllerStartupTimeout)
	err = waitForController(readyCtx, func(ctx context.Context) error { _, err := client.Ping(ctx); return err })
	cancel()
	if err != nil {
		logger.Error("Reclamation controller unavailable", "component", "cli", "operation", "reclaim_storage")
		return 1
	}
	report, err := client.ReclaimLinux(ctx, target)
	if err != nil {
		logger.Error("Reclamation result unavailable", "component", "cli", "operation", "reclaim_storage")
		return 1
	}
	if json.NewEncoder(stdout).Encode(report) != nil {
		logger.Error("Cannot write reclamation result", "component", "cli", "operation", "reclaim_storage")
		return 1
	}
	if !report.Complete() {
		return 1
	}
	return 0
}
