package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

type environmentExportClient interface {
	ExportEnvironment(context.Context, string, io.Writer) (controlapi.EnvironmentExportResult, error)
}

func exportEnvironment(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	language := cliLanguage()
	flags := flag.NewFlagSet("haco env export", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() {
		fmt.Fprintln(diagnostic, language.Format("usage", "haco env export [--json] <stopped-env> [file.haco]"))
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
		fmt.Fprintln(diagnostic, language.Text("error.controller"))
		return 1
	}
	result, err := saveEnvironmentExport(ctx, client, pos[0], destination)
	if err != nil {
		fmt.Fprint(diagnostic, language.Format("env.export.failed", err))
		if result.TemporarySnapshot != "" {
			fmt.Fprint(diagnostic, language.Format("env.export.retained_snapshot", displayCell(string(result.TemporarySnapshot))))
		}
		return 1
	}
	if *jsonOutput {
		err = json.NewEncoder(out).Encode(struct {
			File   string                             `json:"file"`
			Result controlapi.EnvironmentExportResult `json:"result"`
		}{destination, result})
	} else {
		_, err = fmt.Fprint(out, language.Format("env.export.completed", displayCell(pos[0]), displayCell(destination)))
	}
	if err != nil {
		return 1
	}
	return 0
}
