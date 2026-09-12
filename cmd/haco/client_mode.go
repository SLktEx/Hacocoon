package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	controllerClientModeEnv   = "HACO_CLIENT_MODE"
	controllerClientModeValue = "controller"
)

// Controller-client mode is an execution-context guard for the general haco
// binary installed in trusted haco-host. It is not an authorization token: the
// Physical Host controller remains responsible for policy and authority.
func init() {
	if isHacocoonLogin(os.Args[0]) || isVersionInvocation(os.Args[1:]) || isHelpInvocation(os.Args[1:]) {
		return
	}
	handled, err := handleControllerClientModeArgs(
		context.Background(),
		os.Args[1:],
		newDefaultEnvironmentController,
		os.Stdin,
		os.Stdout,
		os.Stderr,
	)
	if !handled {
		return
	}
	if err != nil {
		fail(err)
	}
	os.Exit(0)
}

func handleControllerClientModeArgs(
	ctx context.Context,
	args []string,
	factory environmentControllerFactory,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) (bool, error) {
	active, err := controllerClientMode()
	if err != nil {
		return true, err
	}
	if !active {
		return false, nil
	}
	if strings.TrimSpace(os.Getenv("HACO_CONTROL_SOCKET")) == "" {
		return true, fmt.Errorf("%s=%s requires HACO_CONTROL_SOCKET: %w", controllerClientModeEnv, controllerClientModeValue, core.ErrInvalidArgument)
	}
	if len(args) > 0 && (args[0] == "env" || isGeneralControllerCommand(args[0])) {
		// env_client.go and general_client.go own the first-class controller
		// namespaces. Returning here keeps initialization-order independent while
		// still validating the explicit trusted-host client context above.
		return false, nil
	}
	if len(args) > 1 && args[0] == "host" && args[1] == "shell" {
		// host_client.go owns the controller-backed interactive Host session.
		// Removed bootstrap commands continue through the classification error
		// path below; product haco setup owns controller-backed preparation.
		return false, nil
	}
	if factory == nil || stdin == nil || stdout == nil || stderr == nil {
		return true, core.ErrInvalidArgument
	}
	if isLegacyEnvironmentAlias(args) {
		client, err := factory()
		if err != nil {
			return true, err
		}
		return true, environmentClientCommand(ctx, client, args, stdin, stdout, stderr)
	}

	command := "<none>"
	if len(args) > 0 {
		command = args[0]
	}
	return true, controllerClientModeCommandError(command)
}

func controllerClientModeCommandError(command string) error {
	classification, ok := hacoCommandClassification(command)
	if !ok {
		return fmt.Errorf("command %q is not available in controller-client mode; refusing local composition fallback: %w", command, core.ErrUnsupported)
	}
	switch classification.Domain {
	case commandDomainGeneral:
		return fmt.Errorf("command %q is a general haco client operation but its controller migration is not implemented yet (%s); refusing local composition fallback: %w", command, classification.State, core.ErrUnsupported)
	case commandDomainPhysicalHost:
		return fmt.Errorf("command %q is classified as Physical Host-local (%s) and is unavailable inside controller-client mode: %w", command, classification.State, core.ErrUnsupported)
	case commandDomainTrustedHost:
		return fmt.Errorf("command %q is classified as trusted haco-host-local (%s); the replacement surface is not implemented yet, so refusing local composition fallback: %w", command, classification.State, core.ErrUnsupported)
	case commandDomainCompatibility:
		if classification.Replacement != "" {
			return fmt.Errorf("command %q is a temporary compatibility alias; use %s: %w", command, classification.Replacement, core.ErrUnsupported)
		}
	}
	return fmt.Errorf("command %q is not available in controller-client mode; refusing local composition fallback: %w", command, core.ErrUnsupported)
}

func controllerClientMode() (bool, error) {
	value := strings.TrimSpace(os.Getenv(controllerClientModeEnv))
	switch value {
	case "":
		return false, nil
	case controllerClientModeValue:
		return true, nil
	default:
		return false, fmt.Errorf("unsupported %s value %q: %w", controllerClientModeEnv, value, core.ErrInvalidArgument)
	}
}

func isLegacyEnvironmentAlias(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "create", "status", "exec", "shell", "delete":
		return true
	default:
		return false
	}
}

func isVersionInvocation(args []string) bool {
	return (len(args) == 1 && args[0] == "--version") || (len(args) > 0 && args[0] == "version")
}
