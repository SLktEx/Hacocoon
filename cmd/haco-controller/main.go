package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const controlGroupGIDEnv = "HACO_CONTROL_GROUP_GID"

func main() {
	logger, err := logging.NewFromEnv(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "controller logging configuration is invalid")
		os.Exit(1)
	}
	logging.SetRoot(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	standardEgress, err := controllerMode(os.Args[1:])
	if err != nil {
		fail(err)
	}
	app, err := composition.Controller(ctx)
	if err != nil {
		fail(err)
	}
	server := control.NewServer()
	if err := controlapi.Register(server, app.Environments, app.Clients); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterStop(server, app.Environments); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterRepositories(server, app.Repositories, app.GitBroker); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterOCITransfer(server, app.OCITransfer); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterGeneral(server, app.Bases, app.Runner, app.Events, app.Capabilities); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterHost(server, app.Runtime); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterSetup(server, app); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterDoctor(server, app); err != nil {
		fail(err)
	}

	var proxyListener net.Listener
	if standardEgress {
		prepareCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		address, prepareErr := app.Runtime.PrepareEgressProxy(prepareCtx)
		cancel()
		if prepareErr != nil {
			fail(fmt.Errorf("prepare Standard egress substrate failed"))
		}
		proxyListener, err = net.Listen("tcp4", address)
		if err != nil {
			fail(fmt.Errorf("bind Standard egress endpoint: %w", err))
		}
		defer proxyListener.Close()
	}

	path := control.SocketPath()
	listener, err := controllerListener(path)
	if err != nil {
		fail(err)
	}
	defer listener.Close()
	if err := app.GitBroker.Start(ctx); err != nil {
		fail(err)
	}
	defer app.GitBroker.Close()

	logger = logging.Root().With("component", "control")
	logger.InfoContext(ctx, "controller listening", "socket_path", path)
	services := []func(context.Context) error{func(ctx context.Context) error { return server.Serve(ctx, listener) }}
	if proxyListener != nil {
		services = append(services, func(ctx context.Context) error { return app.EgressProxy.Serve(ctx, proxyListener) })
		logging.Root().InfoContext(ctx, "Standard egress proxy listening", "component", "proxy", "operation", "serve_http")
	}
	if err := serveControllerServices(ctx, services...); err != nil && !errors.Is(err, context.Canceled) {
		fail(err)
	}
}

// The installed unit explicitly enables the replaceable Standard component.
// A bare controller remains available for isolated control-transport use.
func controllerMode(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) == 1 && args[0] == "--standard-egress" {
		return true, nil
	}
	return false, fmt.Errorf("usage: haco-controller [--standard-egress]")
}

func serveControllerServices(parent context.Context, services ...func(context.Context) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if len(services) == 0 {
		return fmt.Errorf("controller has no services")
	}
	results := make(chan error, len(services))
	for _, serve := range services {
		go func() { results <- serve(ctx) }()
	}
	err := <-results
	cancel()
	for range len(services) - 1 {
		<-results
	}
	if parent.Err() != nil {
		return parent.Err()
	}
	// An independently stopped component must restart the whole controller,
	// including when it returned nil or context.Canceled unexpectedly.
	if err == nil || errors.Is(err, context.Canceled) {
		return fmt.Errorf("controller service stopped unexpectedly")
	}
	return err
}

func controllerListener(path string) (net.Listener, error) {
	// Development and tests keep their caller-owned override private. The
	// production endpoint follows the Docker socket model: root owns it, and
	// only members of the installer-managed hacocoon group receive access.
	if strings.TrimSpace(os.Getenv("HACO_CONTROL_SOCKET")) != "" {
		return control.ListenUnix(path, 0o600)
	}
	if path != control.DefaultSocketPath {
		return nil, fmt.Errorf("unexpected production control socket path %q", path)
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("production controller socket requires root authority")
	}

	gid, err := productionControlGroupGID()
	if err != nil {
		return nil, err
	}
	listener, err := control.ListenUnix(path, 0o660)
	if err != nil {
		return nil, err
	}
	if err := os.Chown(path, 0, gid); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("set control socket group owner: %w", err)
	}
	// Chown can clear special permission bits on some filesystems. Re-assert the
	// exact Docker-style socket mode after ownership is final.
	if err := os.Chmod(path, 0o660); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("set control socket permissions: %w", err)
	}
	return listener, nil
}

func productionControlGroupGID() (int, error) {
	value := strings.TrimSpace(os.Getenv(controlGroupGIDEnv))
	if value == "" {
		return 0, fmt.Errorf("%s is required for the production control socket", controlGroupGIDEnv)
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned numeric gid: %w", controlGroupGIDEnv, err)
	}
	if parsed == 0 {
		return 0, fmt.Errorf("%s must not resolve to the root group", controlGroupGIDEnv)
	}
	return int(parsed), nil
}

func fail(err error) {
	logging.Root().Error("controller failed", "component", "control", "error", err)
	os.Exit(1)
}
