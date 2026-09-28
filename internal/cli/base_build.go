package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	packerplugin "github.com/SLktEx/Hacocoon/internal/adapters/packer"
	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func readBaseDefinition(path string) (basebuild.Definition, error) {
	var d basebuild.Definition
	before, err := os.Stat(path)
	if err != nil {
		return d, err
	}
	if !before.Mode().IsRegular() {
		return d, core.ErrInvalidArgument
	}
	file, err := os.Open(path)
	if err != nil {
		return d, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return d, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 4*basebuild.MaxScriptBytes {
		return d, core.ErrInvalidArgument
	}
	decoder := json.NewDecoder(io.LimitReader(file, 4*basebuild.MaxScriptBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return d, core.ErrInvalidArgument
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return d, core.ErrInvalidArgument
	}
	return d, d.Validate()
}
func runBaseBuild(args []string) int {
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(os.Stderr, "haco:", flagErr)
		return 2
	}
	flags := flag.NewFlagSet("base build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	name := flags.String("name", "", cliMessage("base.packer_name"))
	from := flags.String("from", "", cliMessage("base.packer_from"))
	builder := flags.String("builder", "", cliMessage("base.builder_name"))
	maxSize := flags.String("max-image-size", "", cliMessage("base.max_image_size"))
	output := flags.Bool("output", false, cliMessage("base.packer_output"))
	flags.Usage = func() { commandHelp(os.Stderr, "base build", cliLanguage()) }
	if err := flags.Parse(clean); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	var d basebuild.Definition
	var template *basebuild.PackerTemplate
	var err error
	if *name == "" && *from == "" && strings.HasSuffix(flags.Arg(0), ".json") {
		d, err = readBaseDefinition(flags.Arg(0))
	} else {
		d.Name, d.From = core.BaseName(*name), core.BaseName(*from)
		template, err = readPackerContext(flags.Arg(0))
		if err == nil {
			err = (basebuild.ImportRequest{Name: d.Name}).Validate()
		}
	}
	if err == nil && template == nil {
		if *builder != "" {
			d.BuilderName = *builder
		}
		err = d.Validate()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 2
	}
	if template != nil && (*from != "" || *builder != "") {
		fmt.Fprintln(os.Stderr, "haco:", cliMessage("base.packer_hcl_source"))
		return 2
	}
	limit, limitErr := baseArchiveLimit(*maxSize)
	if limitErr != nil || (template == nil && *maxSize != "") {
		fmt.Fprintln(os.Stderr, "haco:", core.ErrInvalidArgument)
		return 2
	}
	client := controlapi.NewDefaultClient()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var result controlapi.BaseBuildResponse
	if template != nil {
		result.Result, err = packerplugin.Build(ctx, *template, basebuild.ImportRequest{Name: d.Name, MaxBytes: limit}, client)
	} else {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		result, err = client.BuildBase(ctx, d)
	}
	if !*output {
		result.Result.Execution = nil
	}
	if *output && !jsonOutput && result.Result.Execution != nil {
		// Explicit private output is quoted so guest terminal controls cannot
		// masquerade as prompts. It never enters the structured error/log chain.
		fmt.Fprintf(os.Stderr, "Packer stdout: %q\nPacker stderr: %q\n", result.Result.Execution.Stdout, result.Result.Execution.Stderr)
		result.Result.Execution = nil
	}
	if e := writeCLIResult(os.Stdout, result.Result, jsonOutput); e != nil {
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		if template != nil && result.Result.Stage != "" {
			fmt.Fprintln(os.Stderr, cliMessage("base.packer_failed"))
		}
		return 1
	}
	return 0
}
