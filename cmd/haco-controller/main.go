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

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const controlGroupGIDEnv = "HACO_CONTROL_GROUP_GID"

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

	path := control.SocketPath()
	listener, err := controllerListener(path)
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
