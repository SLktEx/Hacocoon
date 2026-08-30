package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := composition.Local(ctx)
	if err != nil {
		fail(err)
	}
	server := control.NewServer()
	if err := controlapi.Register(server, app.Environments, app.Clients); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterWorkloads(server, app.Runtime); err != nil {
		fail(err)
	}
	path := control.SocketPath()
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		fail(err)
	}
	defer listener.Close()

	logger := logging.Root().With("component", "control")
	logger.InfoContext(ctx, "controller listening", "socket_path", path)
	if err := server.Serve(ctx, listener); err != nil && !errors.Is(err, context.Canceled) {
		fail(err)
	}
}

func fail(err error) {
	logging.Root().Error("controller failed", "component", "control", "error", err)
	os.Exit(1)
}
