package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
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

	sudoGroup, err := user.LookupGroup("sudo")
	if err != nil {
		return nil, fmt.Errorf("resolve sudo group for local controller clients: %w", err)
	}
	sudoGID, err := strconv.Atoi(sudoGroup.Gid)
	if err != nil || sudoGID < 0 {
		return nil, fmt.Errorf("invalid sudo group gid %q", sudoGroup.Gid)
	}

	runtimeDir := filepath.Dir(privilegedPath)
	if err := os.Chown(runtimeDir, 0, sudoGID); err != nil {
		return nil, fmt.Errorf("set controller runtime directory group: %w", err)
	}
	if err := os.Chmod(runtimeDir, 0o710); err != nil {
		return nil, fmt.Errorf("set controller runtime directory permissions: %w", err)
	}

	privileged, err := control.ListenUnix(privilegedPath, 0o600)
	if err != nil {
		return nil, err
	}
	client, err := control.ListenUnix(clientPath, 0o660)
	if err != nil {
		_ = privileged.Close()
		return nil, err
	}
	if err := os.Chown(clientPath, 0, sudoGID); err != nil {
		_ = client.Close()
		_ = privileged.Close()
		return nil, fmt.Errorf("set local client socket group: %w", err)
	}
	if err := os.Chmod(clientPath, 0o660); err != nil {
		_ = client.Close()
		_ = privileged.Close()
		return nil, fmt.Errorf("set local client socket permissions: %w", err)
	}
	return []net.Listener{privileged, client}, nil
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
