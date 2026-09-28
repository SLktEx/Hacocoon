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
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/cli/bytesize"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type baseImportClient interface {
	ImportBase(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error)
}

func runBaseImport(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return baseImportCommand(ctx, args, os.Stdout, os.Stderr)
}
func baseImportCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco base import", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() { commandHelp(diagnostic, "base import", cliLanguage()) }
	name := flags.String("name", "", cliMessage("base.packer_name"))
	maxSize := flags.String("max-image-size", "", cliMessage("base.max_image_size"))
	machine := flags.Bool("json", false, cliMessage("flag.json"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	limit, limitErr := baseArchiveLimit(*maxSize)
	req := basebuild.ImportRequest{Name: core.BaseName(*name), MaxBytes: limit}
	if limitErr != nil || flags.NArg() != 1 || req.Validate() != nil {
		flags.Usage()
		return 2
	}
	client := controlapi.NewDefaultClient()
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

func baseArchiveLimit(raw string) (int64, error) {
	if raw == "" || raw == "unlimited" {
		return 0, nil
	}
	value, err := bytesize.Parse(raw, uint64(basebuild.MaxArchiveLimitBytes))
	return int64(value), err
}
