package controller

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
	"time"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const controlGroupGIDEnv = "HACO_CONTROL_GROUP_GID"

func Main() {
	logger, err := logging.NewFromEnv(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "controller logging configuration is invalid")
		os.Exit(1)
	}
	logging.SetRoot(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		logging.Root().Error("controller failed", "component", "control", "error", err)
		os.Exit(1)
	}
}

func run(parent context.Context, args []string) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	standardEgress, err := controllerMode(args)
	if err != nil {
		return err
	}
	app, err := composition.Controller(ctx)
	if err != nil {
		return err
	}
	defer app.Networks.Close()
	server := control.NewServer()
	// Register every management contract before publishing the endpoint. Any
	// collision or invalid service prevents the whole controller from serving.
	if err := errors.Join(
		controlapi.RegisterWorkflow(server, app.Workflow),
		controlapi.RegisterNetworkRules(server, app.Networks, app.Configuration),
		controlapi.RegisterNetwork(server, app.Networks),
		controlapi.RegisterReviews(server, app.Reviews),
		controlapi.RegisterCache(server, app.Cache),
		controlapi.RegisterConfiguration(server, app.Configuration),
		controlapi.Register(server, app.Environments, app.Clients),
		controlapi.RegisterForwardStreams(server, app.Clients),
		controlapi.RegisterBaseManage(server, app.BaseManage),
		controlapi.RegisterBaseBuild(server, app.BaseBuild),
		registerBaseImport(server, app),
		registerEnvironmentExport(server, app),
		registerWorkspaceImport(server, app),
		registerEnvironmentImport(server, app),
		controlapi.RegisterEnvironmentCopy(server, app.EnvironmentCopy),
		controlapi.RegisterSnapshotRestore(server, app.SnapshotRestore),
		controlapi.RegisterSnapshots(server, app.Environments),
		controlapi.RegisterEnvironmentStreams(server, app.Clients),
		controlapi.RegisterStart(server, app.Environments),
		controlapi.RegisterStop(server, app.Environments),
		controlapi.RegisterAWS(server, app.AWS),
		controlapi.RegisterManagedWorkspaces(server, app.Environments),
		controlapi.RegisterRepositories(server, app.Repositories, app.GitBroker),
		controlapi.RegisterOCIImages(server, app.OCIImages),
		controlapi.RegisterOCIStores(server, app.PersistentResources),
		controlapi.RegisterGeneral(server, app.Bases, app.Runner, app.Events, app.Capabilities),
		controlapi.RegisterHost(server, app),
		controlapi.RegisterProjectSetup(server, app.ProjectSetup),
		controlapi.RegisterSetup(server, app),
		registerReclamation(server, app),
		controlapi.RegisterDoctor(server, app),
	); err != nil {
		return err
	}

	var proxyListener net.Listener
	if standardEgress {
		executable, sourceErr := os.Executable()
		if sourceErr != nil {
			return sourceErr
		}
		if sourceErr = app.Runtime.ConfigureEnvironmentDNS(filepath.Join(filepath.Dir(executable), "haco")); sourceErr != nil {
			return sourceErr
		}
		prepareCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		address, prepareErr := app.Runtime.PrepareEgressProxy(prepareCtx)
		cancel()
		if prepareErr != nil {
			return fmt.Errorf("prepare Standard egress substrate failed")
		}
		proxyListener, err = net.Listen("tcp4", address)
		if err != nil {
			return fmt.Errorf("bind Standard egress endpoint: %w", err)
		}
		defer func() { _ = proxyListener.Close() }()
	}

	path := control.SocketPath()
	listener, err := controllerListener(path)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	defer app.GitBroker.Close()
	if err := app.GitBroker.Start(ctx); err != nil {
		return err
	}

	logger := logging.Root().With("component", "control")
	logger.InfoContext(ctx, "controller listening", "socket_path", path)
	services := []func(context.Context) error{func(ctx context.Context) error { return server.Serve(ctx, listener) }}
	if proxyListener != nil {
		services = append(services, func(ctx context.Context) error { return app.EgressProxy.Serve(ctx, proxyListener) })
		logging.Root().InfoContext(ctx, "Standard egress proxy listening", "component", "proxy", "operation", "serve_http")
	}
	return serveControllerServices(ctx, services...)
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
