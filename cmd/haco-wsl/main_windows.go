//go:build windows && (amd64 || arm64)

// haco-wsl is an internal Windows installer helper, not another user-facing CLI.
package main

import (
	"context"
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, wslreclaim.EnrollInstallation)
	cancel()
	stop()
	os.Exit(code)
}
