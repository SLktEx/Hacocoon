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

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// runStream is the OpenSSH stdio adapter. Its stdout contains application bytes only.
func runStream(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: haco stream <target>")
		return 2
	}
	target, err := core.DecodeStreamTarget(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: invalid stream target")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: controller unavailable")
		return 1
	}
	ready, stop := context.WithTimeout(ctx, controllerStartupTimeout)
	err = waitForController(ready, func(ctx context.Context) error { _, err := client.Ping(ctx); return err })
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: controller readiness failed")
		return 1
	}
	conn, err := client.OpenEnvironmentStream(ctx, target)
	if err != nil {
		dailyFailure(os.Stderr, "stream", "target", "", err)
		return 1
	}
	defer conn.Close()
	// Closing owned process stdin interrupts its copier when the target exits;
	// neither os/exec nor a goroutine retains the ProxyCommand's pipe afterward.
	input, err := streamInput()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: stream input unavailable")
		return 1
	}
	defer input.Close()
	inputDone := make(chan error, 1)
	stopIO := context.AfterFunc(ctx, func() { _ = conn.Close(); _ = input.Close() })
	defer stopIO()
	go func() {
		_, e := io.Copy(conn, input)
		if e == nil {
			e = conn.(interface{ CloseWrite() error }).CloseWrite()
		}
		if e != nil && !errors.Is(e, os.ErrClosed) {
			cancel()
		}
		inputDone <- e
	}()
	_, err = io.Copy(os.Stdout, conn)
	_ = input.Close()
	_ = conn.(interface{ CloseWrite() error }).CloseWrite()
	<-inputDone
	if err != nil || ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "haco: stream disconnected")
		return 1
	}
	waitCtx, finish := context.WithTimeout(ctx, 5*time.Second)
	defer finish()
	if err = conn.(interface{ Wait(context.Context) error }).Wait(waitCtx); err != nil {
		fmt.Fprintln(os.Stderr, "haco: stream failed")
		return 1
	}
	return 0
}
