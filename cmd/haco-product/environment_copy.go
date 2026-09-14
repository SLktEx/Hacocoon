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
	language := cliLanguage()
	flags := flag.NewFlagSet("haco env copy", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	machine := flags.Bool("json", false, language.Text("flag.json"))
	flags.Usage = func() {
		fmt.Fprintln(diagnostic, language.Format("usage", "haco env copy [--json] <stopped-env> [new-env]"))
	}
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
		fmt.Fprintln(diagnostic, language.Text("error.controller"))
		return 1
	}
	response, err := client.CopyEnvironment(ctx, req)
	var outputErr error
	if *machine {
		outputErr = json.NewEncoder(out).Encode(response.Result)
	} else if response.Result.Environment != "" {
		_, outputErr = fmt.Fprint(out, language.Format("env.copy.result", displayCell(response.Result.Environment), displayCell(string(response.Result.State))))
		if response.Result.Workspace != "" && outputErr == nil {
			_, outputErr = fmt.Fprint(out, language.Format("env.copy.workspace", displayCell(string(response.Result.Workspace))))
		}
		if response.Result.OCI != "" && outputErr == nil {
			_, outputErr = fmt.Fprint(out, language.Format("env.copy.oci", displayCell(string(response.Result.OCI))))
		}
		if response.Result.TemporarySnapshot != "" && outputErr == nil {
			_, outputErr = fmt.Fprint(out, language.Format("env.copy.retained_snapshot", displayCell(string(response.Result.TemporarySnapshot))))
		}
	}
	if outputErr != nil {
		fmt.Fprintln(diagnostic, language.Text("error.write_result"))
		return 1
	}
	if err != nil {
		fmt.Fprint(diagnostic, language.Format("env.copy.failed", err))
		return 1
	}
	return 0
}
