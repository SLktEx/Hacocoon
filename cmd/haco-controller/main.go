package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := composition.Local(ctx)
	if err != nil {
		fail(err)
	}
	server := control.NewServer()
	if err := controlapi.Register(server, app.Environments); err != nil {
		fail(err)
	}
	path := control.SocketPath()
	listener, err := control.ListenUnix(path, 0o660)
	if err != nil {
		fail(err)
	}
	defer listener.Close()
	fmt.Fprintf(os.Stderr, "haco-controller: listening on %s\n", path)
	if err := server.Serve(ctx, listener); err != nil && !errors.Is(err, context.Canceled) {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "haco-controller:", err)
	os.Exit(1)
}
