package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"io"
)

type environmentExportClient interface {
	ExportEnvironment(context.Context, string, io.Writer) (controlapi.EnvironmentExportResult, error)
}

func exportEnvironment(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco env export", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() { fmt.Fprintln(diagnostic, "Usage: haco env export [--json] <stopped-env> [file.haco]") }
	jsonOutput := flags.Bool("json", false, "machine-readable result")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	pos := flags.Args()
	if len(pos) < 1 || len(pos) > 2 {
		fmt.Fprintln(diagnostic, "Usage: haco env export [--json] <stopped-env> [file.haco]")
		return 2
	}
	if err := (controlapi.EnvironmentExportRequest{Source: pos[0]}).Validate(); err != nil {
		flags.Usage()
		return 2
	}
	destination := pos[0] + ".haco"
	if len(pos) == 2 {
		destination = pos[1]
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot open controller client")
		return 1
	}
	result, err := saveEnvironmentExport(ctx, client, pos[0], destination)
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: export failed: %v\n", err)
		if result.TemporarySnapshot != "" {
			fmt.Fprintf(diagnostic, "Temporary snapshot retained: %s\n", result.TemporarySnapshot)
		}
		return 1
	}
	if *jsonOutput {
		err = json.NewEncoder(out).Encode(struct {
			File   string                             `json:"file"`
			Result controlapi.EnvironmentExportResult `json:"result"`
		}{destination, result})
	} else {
		_, err = fmt.Fprintf(out, "Exported %s to %s\n", pos[0], destination)
	}
	if err != nil {
		return 1
	}
	return 0
}
