package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/streamio"
)

// The Windows client invokes this fixed operation in its selected local WSL.
// No caller-supplied socket, command, environment override, or authority upgrade.
func runControlStdio() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	if err := streamio.BridgeStdio(ctx, os.Stdin, os.Stdout, control.UnixDialer(control.DefaultSocketPath)); err != nil {
		return 1
	}
	return 0
}
