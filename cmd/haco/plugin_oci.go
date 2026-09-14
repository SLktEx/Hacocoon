package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

func ociPluginCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "status", "docker":
		default:
			return fmt.Errorf("unknown OCI plugin command %q: %w", args[0], core.ErrInvalidArgument)
		}
	}
	if app == nil || app.OCI == nil {
		return fmt.Errorf("OCI plugin is disabled; set HACO_PLUGIN_OCI=nerdctl or HACO_PLUGIN_OCI=docker: %w", core.ErrRuntimeUnavailable)
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci <status|docker> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("usage: haco plugin oci status: %w", core.ErrInvalidArgument)
		}
		fmt.Printf("driver: %s\n", app.OCI.Driver())
		return nil
	case "docker":
		return ociDockerCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown OCI plugin command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func ociDockerCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: haco plugin oci docker <status|prepare> <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	action, environment := args[0], args[1]
	jsonOutput := false
	if len(args) == 3 {
		if args[2] != "--json" {
			return fmt.Errorf("unknown OCI Docker option %q: %w", args[2], core.ErrInvalidArgument)
		}
		jsonOutput = true
	}

	var (
		status ociplugin.DockerCompatibilityStatus
		err    error
	)
	switch action {
	case "status":
		status, err = app.OCI.DockerStatus(ctx, environment)
	case "prepare":
		status, err = app.OCI.PrepareDocker(ctx, environment)
	default:
		return fmt.Errorf("unknown OCI Docker command %q: %w", action, core.ErrInvalidArgument)
	}
	if jsonOutput {
		if encodeErr := json.NewEncoder(os.Stdout).Encode(status); encodeErr != nil {
			return encodeErr
		}
	} else {
		printOCIDockerStatus(status)
	}
	return err
}

func printOCIDockerStatus(status ociplugin.DockerCompatibilityStatus) {
	if status.Environment != "" {
		fmt.Printf("environment: %s\n", status.Environment)
	}
	fmt.Printf("docker-cli: %t\n", status.DockerCLI)
	fmt.Printf("dockerd: %t\n", status.DockerDaemon)
	fmt.Printf("containerd: %t (active=%t)\n", status.Containerd, status.ContainerdActive)
	fmt.Printf("systemd: %t\n", status.Systemd)
	fmt.Printf("docker-group: %t\n", status.DockerGroup)
	fmt.Printf("units: socket=%t service=%t\n", status.SocketUnitVerified, status.ServiceUnitVerified)
	fmt.Printf("haco-socket: enabled=%t active=%t\n", status.SocketEnabled, status.SocketActive)
	fmt.Printf("engine: active=%t (on-demand is expected)\n", status.EngineActive)
	fmt.Printf("vendor-docker: enabled=%t active=%t\n", status.VendorDockerEnabled, status.VendorDockerActive)
	fmt.Printf("ready: %t\n", status.Ready)
}
