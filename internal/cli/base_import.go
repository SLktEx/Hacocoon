package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type baseImportClient interface {
	ImportBase(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error)
}

func runBaseImport(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	return baseImportCommand(ctx, args, os.Stdout, os.Stderr)
}
func baseImportCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco base import", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() { commandHelp(diagnostic, "base import", cliLanguage()) }
	name := flags.String("name", "", cliMessage("base.packer_name"))
	machine := flags.Bool("json", false, cliMessage("flag.json"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	req := basebuild.ImportRequest{Name: core.BaseName(*name)}
	if flags.NArg() != 1 || req.Validate() != nil {
		flags.Usage()
		return 2
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("error.controller"))
		return 1
	}
	result, err := loadBaseImport(ctx, client, flags.Arg(0), req)
	if *machine {
		if writeErr := json.NewEncoder(out).Encode(result); writeErr != nil {
			return 1
		}
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("base.import.failed"))
		_, _ = fmt.Fprintf(diagnostic, "haco: %v\n", err)
		if result.Builder != "" {
			_, _ = fmt.Fprintf(diagnostic, cliLanguage().Text("base.import.retained"), result.Builder, result.State)
		}
		return 1
	}
	if !*machine {
		if _, err := fmt.Fprintf(out, cliLanguage().Text("base.import.ready"), result.Base.Name, result.Base.Revision, result.Base.Name); err != nil {
			return 1
		}
	}
	return 0
}
