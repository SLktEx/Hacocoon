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
		fmt.Fprintln(diagnostic, "Usage: haco env create --workspace <controller-path> <name> | list | status <name> | ssh --key <public-key-file> --port <port> <name> | disconnect <name> <connection-id> | stop <name>")
		return 2
	}
	if len(args) == 0 {
		return usage()
	}
	if args[0] == "--help" || args[0] == "-h" {
		usage()
		return 0
	}
	flags := flag.NewFlagSet("haco env "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	var workspace, keyPath string
	var port int
	switch args[0] {
	case "create":
		flags.StringVar(&workspace, "workspace", "", "Workspace path on the controller")
	case "ssh":
		flags.StringVar(&keyPath, "key", "", "client-owned SSH public key file")
		flags.IntVar(&port, "port", 2222, "loopback port on the WSL Physical Host")
	case "list", "status", "disconnect", "stop":
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
		result, err = client.CreateEnvironment(ctx, controlapi.EnvironmentCreateRequest{Name: pos[0], WorkspacePath: workspace})
	case "list":
		result, err = client.ListEnvironments(ctx)
	case "status":
		result, err = client.EnvironmentStatus(ctx, pos[0])
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
