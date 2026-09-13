package main

import (
	"context"
	"errors"
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
	if len(args) > 0 && args[0] == "tunnel" {
		return environmentCommand(ctx, args, os.Stdout, os.Stderr)
	}
	timeout := 15 * time.Minute
	if len(args) > 0 && args[0] == "import" {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return environmentCommand(ctx, args, os.Stdout, os.Stderr)
}

func environmentCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if requestedCommandHelp(append([]string{"env"}, args...), out) {
		return 0
	}
	usage := func() int {
		commandHelp(diagnostic, "env", cliLanguage())
		return 2
	}
	if len(args) == 0 {
		return usage()
	}
	if args[0] == "tunnel" {
		return forwardClientCommand(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "import" {
		return importEnvironment(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "export" {
		return exportEnvironment(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "copy" {
		return copyEnvironment(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "switch-base" {
		fmt.Fprintln(diagnostic, cliMessage("env.switch_base_disabled"))
		return 2
	}
	if args[0] == "--help" || args[0] == "-h" {
		usage()
		return 0
	}
	flags := flag.NewFlagSet("haco env "+args[0], flag.ContinueOnError)
	configureCLIFlags(flags, diagnostic)
	var workspace, keyPath, base, resource, protocol string
	var targetPort int
	var port int
	var jsonOutput, noOCI bool
	switch args[0] {
	case "forward":
		flags.BoolVar(&jsonOutput, "json", false, cliMessage("flag.json"))
		flags.StringVar(&protocol, "protocol", "tcp", cliMessage("detail.protocol"))
		flags.IntVar(&port, "port", 0, cliMessage("flag.ssh_port"))
		flags.IntVar(&targetPort, "target-port", 0, cliMessage("detail.target_port"))
	case "create":
		flags.BoolVar(&jsonOutput, "json", false, cliMessage("flag.json"))
		flags.BoolVar(&noOCI, "no-oci", false, cliMessage("flag.no_oci"))
		flags.StringVar(&workspace, "workspace", "", cliMessage("flag.workspace"))
		flags.StringVar(&base, "base", "", cliMessage("flag.base"))
		flags.StringVar(&resource, "resource", "", cliMessage("flag.resource"))
	case "ssh":
		flags.BoolVar(&jsonOutput, "json", false, cliMessage("flag.json"))
		flags.StringVar(&keyPath, "key", "", cliMessage("flag.ssh_key"))
		flags.IntVar(&port, "port", 0, cliMessage("flag.ssh_port"))
	case "ssh-config":
	case "status", "list":
		flags.BoolVar(&jsonOutput, "json", false, cliMessage("flag.json"))
	case "disconnect", "start", "stop", "delete":
		flags.BoolVar(&jsonOutput, "json", false, cliMessage("flag.json"))
	default:
		return usage()
	}
	flags.Usage = func() { usage() }
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
		fmt.Fprintln(diagnostic, cliMessage("error.controller"))
		return 1
	}
	mutating := args[0] == "create" || args[0] == "start" || args[0] == "stop" || args[0] == "delete" || args[0] == "ssh" || args[0] == "disconnect"
	if args[0] == "delete" {
		if _, err := fmt.Fprintf(diagnostic, cliLanguage().Text("daily.delete"), pos[0]); err != nil {
			return 1
		}
	}
	if mutating {
		fmt.Fprintf(diagnostic, "[running] environment_%s target=%q\n", args[0], pos[0])
	}
	var result any
	switch args[0] {
	case "forward":
		result, err = client.ForwardEnvironment(ctx, pos[0], core.LocalPortRequest{Protocol: protocol, HostPort: port, TargetPort: targetPort})
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
				fmt.Fprintln(diagnostic, cliMessage("error.write_result"))
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
		name := ""
		if len(pos) > 0 {
			name = pos[0]
		}
		return dailyFailure(diagnostic, "environment_"+args[0], "controller", name, err)
	}
	if mutating {
		fmt.Fprintf(diagnostic, "[succeeded] environment_%s\n", args[0])
		if configEnvironmentName.MatchString(pos[0]) {
			switch args[0] {
			case "create", "start":
				fmt.Fprintf(diagnostic, cliLanguage().Text("daily.next_open"), pos[0], pos[0])
			case "stop":
				fmt.Fprintf(diagnostic, cliLanguage().Text("daily.resume"), pos[0], pos[0])
			case "delete":
				fmt.Fprintln(diagnostic, cliLanguage().Text("daily.retained"))
			}
		}
	}
	if err := writeCLIResult(out, result, jsonOutput); err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.write_result"))
		return 1
	}
	return 0
}
