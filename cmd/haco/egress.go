package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func egressCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 1 || args[0] != "serve" {
		return fmt.Errorf("usage: haco egress serve: %w", core.ErrInvalidArgument)
	}
	if app == nil || app.Runtime == nil || app.EgressProxy == nil {
		return core.ErrRuntimeUnavailable
	}
	address, err := app.Runtime.PrepareEgressProxy(ctx)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen for Hacocoon egress proxy on %s: %w", address, err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler:           app.EgressProxy,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
