package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func statusCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 1 || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		return fmt.Errorf("usage: haco status <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	status, err := app.Clients.Status(ctx, args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		payload, err := json.Marshal(status)
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	fmt.Printf("name: %s\nstate: %s\nruntime: %s\nworkspace: %s\naccess: %s\n",
		status.Environment.Name, status.State, status.Environment.RuntimeRef,
		status.Environment.Workspace.Path, status.Environment.AccessMode)
	if status.Environment.Base != nil {
		fmt.Printf("base: %s\nbase-revision: %s\n", status.Environment.Base.Name, status.Environment.Base.Revision)
	}
	fmt.Printf("cpu: %s\nmemory-bytes: %s\npids: %s\nroot-bytes: %s\n",
		resourceLimitText(status.Environment.Resources.CPU),
		resourceLimitText(status.Environment.Resources.MemoryBytes),
		resourceLimitText(status.Environment.Resources.PIDs),
		resourceLimitText(status.Environment.Resources.RootBytes))
	return nil
}

func connectionsCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 1 || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		return fmt.Errorf("usage: haco connections <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	connections, err := app.Clients.Connections(ctx, args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		payload, err := json.Marshal(connections)
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	for _, connection := range connections {
		fmt.Printf("%s\t%s\t%s:%d\t->\t%d\n", connection.ID, connection.Kind, connection.Host, connection.Port, connection.TargetPort)
	}
	return nil
}

func forwardCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 5 || args[1] != "--host-port" || args[3] != "--target-port" {
		return fmt.Errorf("usage: haco forward <environment> --host-port <port> --target-port <port>: %w", core.ErrInvalidArgument)
	}
	hostPort, err := parsePort(args[2])
	if err != nil {
		return err
	}
	targetPort, err := parsePort(args[4])
	if err != nil {
		return err
	}
	connection, err := app.Clients.Forward(ctx, args[0], core.LocalPortRequest{Protocol: "tcp", HostPort: hostPort, TargetPort: targetPort})
	if err != nil {
		return err
	}
	fmt.Printf("%s\ttcp://%s:%d\t->\t%d\n", connection.ID, connection.Host, connection.Port, connection.TargetPort)
	return nil
}

func unforwardCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: haco unforward <environment> <connection-id>: %w", core.ErrInvalidArgument)
	}
	return app.Clients.Unforward(ctx, args[0], args[1])
}

func sshCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 5 || args[1] != "--public-key" || args[3] != "--host-port" {
		return fmt.Errorf("usage: haco ssh <environment> --public-key <path> --host-port <port>: %w", core.ErrInvalidArgument)
	}
	key, err := os.ReadFile(args[2])
	if err != nil {
		return fmt.Errorf("read SSH public key: %w", err)
	}
	hostPort, err := parsePort(args[4])
	if err != nil {
		return err
	}
	connection, err := app.Clients.SSH(ctx, args[0], core.SSHAccessRequest{PublicKey: string(key), HostPort: hostPort})
	if err != nil {
		return err
	}
	fmt.Println(connection.Command)
	return nil
}

func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %q: %w", raw, core.ErrInvalidArgument)
	}
	return port, nil
}
