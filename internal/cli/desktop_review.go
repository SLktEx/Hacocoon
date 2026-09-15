package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/client/review"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
)

func runDesktopReview() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	// Unblock a reader on cancellation/expiry. This process owns its stdin.
	go func() { <-ctx.Done(); _ = os.Stdin.Close() }()
	client, err := controlapi.NewDefaultClient()
	if err == nil {
		err = serveDesktopReview(ctx, client, os.Stdin, os.Stdout)
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Local review ended without a confirmed result. Inspect current requests and Policy before retrying.")
		return 1
	}
	return 0
}

func serveDesktopReview(ctx context.Context, client *controlapi.Client, input io.Reader, output io.Writer) error {
	if err := waitForControllerClient(ctx, client); err != nil {
		return err
	}
	return (&desktopreview.Session{Client: client}).Serve(ctx, input, output)
}
