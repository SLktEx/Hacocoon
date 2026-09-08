package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func runEnvironment(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return environmentCommand(ctx, args, os.Stdout, os.Stderr)
}

func environmentCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	usage := func() int {
		fmt.Fprintln(diagnostic, "Usage: haco env create --workspace <controller-path> [--base <base>] [--resource oci:<store> | --no-oci] <name> | list [--json] | status [--json] <name> | ssh --key <public-key-file> [--port <port>] <name> | ssh-config <name> | disconnect <name> <connection-id> | copy [--json] <stopped-env> [new-env] | start <name> | stop <name> | delete <name>")
		return 2
	}
	if len(args) == 0 {
		return usage()
	}
	if args[0] == "copy" {
		return copyEnvironment(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "switch-base" {
		fmt.Fprintln(diagnostic, "haco: switch-base is currently disabled; its need and UX will be reconsidered in Stage D or later")
		return 2
	}
	if args[0] == "--help" || args[0] == "-h" {
		usage()
		return 0
	}
	flags := flag.NewFlagSet("haco env "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	var workspace, keyPath, base, resource string
	var port int
	var jsonOutput, noOCI bool
	switch args[0] {
	case "create":
		flags.BoolVar(&noOCI, "no-oci", false, "skip automatic OCI Store copy and attachment")
		flags.StringVar(&workspace, "workspace", "", "Workspace path on the controller")
		flags.StringVar(&base, "base", "", "logical Base name")
		flags.StringVar(&resource, "resource", "", "persistent resource to attach exclusively, e.g. oci:dev")
	case "ssh":
		flags.StringVar(&keyPath, "key", "", "client-owned SSH public key file")
		flags.IntVar(&port, "port", 0, "Physical Host loopback port (default: automatic)")
	case "ssh-config":
	case "status", "list":
		flags.BoolVar(&jsonOutput, "json", false, "machine-readable result")
	case "disconnect", "start", "stop", "delete":
	default:
		return usage()
	}
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	pos := flags.Args()
	n := 1
	if args[0] == "list" {
		n = 0
	}
	if args[0] == "disconnect" {
		n = 2
	}
	if len(pos) != n || (args[0] == "create" && workspace == "") || (args[0] == "ssh" && keyPath == "") {
		return usage()
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot open controller client")
		return 1
	}
	var result any
	switch args[0] {
	case "create":
		result, err = client.CreateEnvironment(ctx, controlapi.EnvironmentCreateRequest{Name: pos[0], WorkspacePath: workspace, Base: core.BaseName(base), PersistentResource: resource, SkipDefaultResource: noOCI})
	case "delete":
		err = client.DeleteEnvironment(ctx, pos[0])
		result = "Environment deleted; Workspace and persistent resources retained"
	case "list":
		var environments []core.Environment
		environments, err = client.ListEnvironments(ctx)
		if err == nil && !jsonOutput {
			if err := writeEnvironmentList(out, environments); err != nil {
				fmt.Fprintln(diagnostic, "haco: cannot write result")
				return 1
			}
			return 0
		}
		result = environments
	case "status":
		var status core.EnvironmentStatus
		status, err = client.EnvironmentStatus(ctx, pos[0])
		if err == nil && !jsonOutput {
			return writeEnvironmentStatus(out, status)
		}
		result = status
	case "ssh-config":
		var connections []core.ClientConnection
		connections, err = client.EnvironmentConnections(ctx, pos[0])
		if err == nil {
			err = writeSSHConfig(out, pos[0], connections)
			if err == nil {
				return 0
			}
		}
	case "start":
		err = client.StartEnvironment(ctx, pos[0])
		result = "Environment running; Workspace and persistent resources retained"
	case "stop":
		err = client.StopEnvironment(ctx, pos[0])
		result = "Environment stopped; Workspace retained"
	case "disconnect":
		err = client.UnforwardEnvironment(ctx, pos[0], pos[1])
		result = "Connection revoked"
	case "ssh":
		var key []byte
		key, err = os.ReadFile(keyPath)
		if err == nil {
			result, err = client.PrepareEnvironmentSSH(ctx, pos[0], core.SSHAccessRequest{PublicKey: string(key), HostPort: port})
		}
	}
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot write result")
		return 1
	}
	return 0
}
