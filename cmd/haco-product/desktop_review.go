package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/desktopreview"
)

func runDesktopReview() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	// Unblock a reader on cancellation/expiry. This process owns its stdin.
	go func() { <-ctx.Done(); os.Stdin.Close() }()
	client, err := controlapi.NewDefaultClient()
	if err == nil {
		err = (&desktopreview.Session{Client: client}).Serve(ctx, os.Stdin, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Local review ended without a confirmed result. Inspect current requests and Policy before retrying.")
		return 1
	}
	return 0
}
