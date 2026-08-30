package main

import (
	"context"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func networkCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 2 || args[0] != "serve" {
		return fmt.Errorf("usage: haco network serve <environment>: %w", core.ErrInvalidArgument)
	}
	if app == nil || app.Egress == nil || app.Bases == nil || app.Clients == nil {
		return core.ErrRuntimeUnavailable
	}
	status, err := app.Clients.Status(ctx, args[1])
	if err != nil {
		return err
	}
	if status.State != core.EnvironmentRunning {
		return fmt.Errorf("environment %q is %s; egress broker requires a running environment: %w", args[1], status.State, core.ErrIncompatibleState)
	}
	return app.Egress.Serve(ctx, status.Environment, func(socketPath string) error {
		if err := app.Bases.EnsureEgressProxy(ctx, status.Environment.RuntimeRef, socketPath); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "egress broker ready for %s: HTTP_PROXY=http://127.0.0.1:3128 HTTPS_PROXY=http://127.0.0.1:3128\n", status.Environment.Name)
		return nil
	})
}
