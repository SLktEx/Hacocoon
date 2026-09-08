package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func copyEnvironment(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco env copy", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	machine := flags.Bool("json", false, "machine-readable result")
	flags.Usage = func() { fmt.Fprintln(diagnostic, "Usage: haco env copy [--json] <stopped-env> [new-env]") }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	pos := flags.Args()
	if len(pos) < 1 || len(pos) > 2 {
		flags.Usage()
		return 2
	}
	req := controlapi.EnvironmentCopyRequest{Source: pos[0]}
	if len(pos) == 2 {
		req.Target = pos[1]
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot open controller client")
		return 1
	}
	response, err := client.CopyEnvironment(ctx, req)
	var outputErr error
	if *machine {
		outputErr = json.NewEncoder(out).Encode(response.Result)
	} else if response.Result.Environment != "" {
		_, outputErr = fmt.Fprintf(out, "Environment %s: %s\n", response.Result.Environment, response.Result.State)
		if response.Result.Workspace != "" && outputErr == nil {
			_, outputErr = fmt.Fprintf(out, "Workspace: %s\n", response.Result.Workspace)
		}
		if response.Result.OCI != "" && outputErr == nil {
			_, outputErr = fmt.Fprintf(out, "OCI: %s\n", response.Result.OCI)
		}
		if response.Result.TemporarySnapshot != "" && outputErr == nil {
			_, outputErr = fmt.Fprintf(out, "Temporary snapshot retained: %s; inspect haco snapshot list before explicit deletion\n", response.Result.TemporarySnapshot)
		}
	}
	if outputErr != nil {
		fmt.Fprintln(diagnostic, "haco: cannot write result")
		return 1
	}
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: Environment copy failed: %v\n", err)
		return 1
	}
	return 0
}
