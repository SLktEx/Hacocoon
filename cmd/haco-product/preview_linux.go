//go:build linux

package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/sshclient"
)

type previewController interface {
	ListEnvironments(context.Context) ([]core.Environment, error)
	StartEnvironment(context.Context, string) error
	EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error)
	ForwardEnvironment(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error)
	UnforwardEnvironment(context.Context, string, string) error
}

func preview(ctx context.Context, c previewController, name string, port int, closeConnection bool) (string, error) {
	if port < 1 || port > 65535 {
		return "", core.ErrInvalidArgument
	}
	if name == "" {
		envs, err := c.ListEnvironments(ctx)
		if err != nil {
			return "", err
		}
		if len(envs) != 1 {
			return "", fmt.Errorf("select an Environment by name; use haco env list")
		}
		name = envs[0].Name
	}
	if !closeConnection {
		if err := c.StartEnvironment(ctx, name); err != nil {
			return "", err
		}
	}
	connections, err := c.EnvironmentConnections(ctx, name)
	if err != nil {
		return "", err
	}
	var matched []core.ClientConnection
	for _, connection := range connections {
		if connection.Kind == "tcp" && connection.TargetPort == port {
			if connection.Host != "127.0.0.1" || connection.Port < 1 || connection.Port > 65535 ||
				connection.ID != fmt.Sprintf("tcp-%d-%d", connection.Port, port) {
				return "", core.ErrIncompatibleState
			}
			matched = append(matched, connection)
		}
	}
	if closeConnection {
		for _, connection := range matched {
			if err := c.UnforwardEnvironment(ctx, name, connection.ID); err != nil {
				return "", err
			}
		}
		return "", nil
	}
	if len(matched) > 1 {
		return "", fmt.Errorf("multiple preview connections; close this port and reopen")
	}
	var connection core.ClientConnection
	if len(matched) == 1 {
		connection = matched[0]
	} else {
		connection, err = c.ForwardEnvironment(ctx, name, core.LocalPortRequest{Protocol: "tcp", TargetPort: port})
		if err != nil {
			return "", err
		}
	}
	if connection.Host != "127.0.0.1" || connection.Port < 1 || connection.Port > 65535 || connection.TargetPort != port ||
		connection.Kind != "tcp" || connection.ID != fmt.Sprintf("tcp-%d-%d", connection.Port, port) {
		return "", core.ErrIncompatibleState
	}
	return fmt.Sprintf("http://127.0.0.1:%d/", connection.Port), nil
}

func openPreview(name string, port int, closeConnection, noBrowser bool, out, diagnostic io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot open controller client")
		return 1
	}
	url, err := preview(ctx, client, name, port, closeConnection)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: preview:", err)
		return 1
	}
	if closeConnection {
		fmt.Fprintln(out, "Preview connection closed.")
		return 0
	}
	if _, err = fmt.Fprintln(out, url); err != nil {
		return 1
	}
	if noBrowser {
		return 0
	}
	desktop, err := sshclient.ResolveDesktop(ctx)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: open the printed URL in your browser")
		return 1
	}
	var command *exec.Cmd
	if desktop.Windows {
		// URL contains only a fixed loopback host and a validated numeric port.
		command = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Process '"+url+"'")
	} else {
		command = exec.Command("xdg-open", url)
	}
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err = command.Start(); err != nil {
		fmt.Fprintln(diagnostic, "haco: open the printed URL in your browser")
		return 1
	}
	if err = command.Process.Release(); err != nil {
		return 1
	}
	return 0
}
