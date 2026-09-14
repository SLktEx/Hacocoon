package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

type environmentImportClient interface {
	ImportEnvironment(context.Context, io.Reader, string) (environmenttransfer.ImportResult, error)
}

func importEnvironment(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	language := cliLanguage()
	flags := flag.NewFlagSet("haco env import", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() {
		fmt.Fprintln(diagnostic, language.Format("usage", "haco env import [--json] <file.haco> [new-env]"))
	}
	jsonOutput := flags.Bool("json", false, language.Text("flag.json"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	pos := flags.Args()
	if len(pos) < 1 || len(pos) > 2 {
		flags.Usage()
		return 2
	}
	name := ""
	if len(pos) == 2 {
		name = pos[1]
	}
	if pos[0] == "" || (controlapi.EnvironmentImportRequest{Environment: name}).Validate() != nil {
		flags.Usage()
		return 2
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, language.Text("error.controller"))
		return 1
	}
	result, err := loadEnvironmentImport(ctx, client, pos[0], name)
	if *jsonOutput {
		if e := json.NewEncoder(out).Encode(result); e != nil {
			return 1
		}
	}
	if err != nil {
		fmt.Fprint(diagnostic, language.Format("env.import.failed", err))
		if result.Environment != "" {
			fmt.Fprint(diagnostic, language.Format("env.import.destination", displayCell(result.Environment), displayCell(string(result.State))))
		}
		if result.Workspace != "" {
			fmt.Fprint(diagnostic, language.Format("env.import.retained_workspace", displayCell(string(result.Workspace))))
		}
		if result.OCI != "" {
			fmt.Fprint(diagnostic, language.Format("env.import.retained_oci", displayCell(string(result.OCI))))
		}
		return 1
	}
	if !*jsonOutput {
		if _, err := fmt.Fprint(out, language.Format("env.import.completed", displayCell(result.Environment))); err != nil {
			return 1
		}
		if len(result.Offline) > 0 {
			if _, err := fmt.Fprint(out, language.Format("env.import.offline", displayCell(strings.Join(result.Offline, ", ")))); err != nil {
				return 1
			}
		}
	}
	return 0
}
