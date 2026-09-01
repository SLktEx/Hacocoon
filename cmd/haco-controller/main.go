package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const (
	controlClientUIDEnv = "HACO_CONTROL_CLIENT_UID"
	controlClientGIDEnv = "HACO_CONTROL_CLIENT_GID"
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
	if err := controlapi.RegisterGeneral(server, app.Bases, app.Runner, app.Events, app.Capabilities); err != nil {
		fail(err)
	}
	if err := controlapi.RegisterHost(server, app.Runtime); err != nil {
		fail(err)
	}

	clientPath := control.SocketPath()
	privilegedPath := control.PrivilegedSocketPath()
	listeners, err := controllerListeners(clientPath, privilegedPath)
	if err != nil {
		fail(err)
	}
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()

	logger := logging.Root().With("component", "control")
	logger.InfoContext(ctx, "controller listening", "client_socket_path", clientPath, "privileged_socket_path", privilegedPath)
	if err := serveAll(ctx, server, listeners); err != nil && !errors.Is(err, context.Canceled) {
		fail(err)
	}
}

func controllerListeners(clientPath, privilegedPath string) ([]net.Listener, error) {
	// Development and test overrides intentionally use one caller-owned private
	// socket. Production uses distinct local-client and privileged endpoints.
	if clientPath == privilegedPath {
		listener, err := control.ListenUnix(clientPath, 0o600)
		if err != nil {
			return nil, err
		}
		return []net.Listener{listener}, nil
	}
	if filepath.Dir(clientPath) != filepath.Dir(privilegedPath) {
		return nil, fmt.Errorf("client and privileged control sockets must share a runtime directory")
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("production controller sockets require root authority")
	}

	clientUID, clientGID, err := productionClientOwner()
	if err != nil {
		return nil, err
	}

	// The runtime directory itself carries no client authority. It only permits
	// pathname traversal so the explicitly-owned 0600 client socket can be
	// reached. The privileged socket remains root-owned 0600.
	runtimeDir := filepath.Dir(privilegedPath)
	if err := os.Chown(runtimeDir, 0, 0); err != nil {
		return nil, fmt.Errorf("set controller runtime directory owner: %w", err)
	}
	if err := os.Chmod(runtimeDir, 0o711); err != nil {
		return nil, fmt.Errorf("set controller runtime directory permissions: %w", err)
	}

	privileged, err := control.ListenUnix(privilegedPath, 0o600)
	if err != nil {
		return nil, err
	}
	client, err := control.ListenUnix(clientPath, 0o600)
	if err != nil {
		_ = privileged.Close()
		return nil, err
	}
	if err := os.Chown(clientPath, clientUID, clientGID); err != nil {
		_ = client.Close()
		_ = privileged.Close()
		return nil, fmt.Errorf("set local client socket owner: %w", err)
	}
	if err := os.Chmod(clientPath, 0o600); err != nil {
		_ = client.Close()
		_ = privileged.Close()
		return nil, fmt.Errorf("set local client socket permissions: %w", err)
	}
	return []net.Listener{privileged, client}, nil
}

func productionClientOwner() (int, int, error) {
	uid, err := parseIDEnv(controlClientUIDEnv)
	if err != nil {
		return 0, 0, err
	}
	gid, err := parseIDEnv(controlClientGIDEnv)
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil
}

func parseIDEnv(name string) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, fmt.Errorf("%s is required for the production local-client socket", name)
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned numeric id: %w", name, err)
	}
	return int(parsed), nil
}

func serveAll(ctx context.Context, server *control.Server, listeners []net.Listener) error {
	if len(listeners) == 0 {
		return fmt.Errorf("controller has no listeners")
	}
	if len(listeners) == 1 {
		return server.Serve(ctx, listeners[0])
	}

	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, len(listeners))
	for _, listener := range listeners {
		listener := listener
		go func() { errs <- server.Serve(serveCtx, listener) }()
	}
	first := <-errs
	cancel()
	for range listeners[1:] {
		<-errs
	}
	return first
}

func fail(err error) {
	logging.Root().Error("controller failed", "component", "control", "error", err)
	os.Exit(1)
}
