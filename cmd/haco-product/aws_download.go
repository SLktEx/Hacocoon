package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"io"
)

type awsDownloadClient interface {
	DownloadS3(context.Context, awsplugin.GetSpec, io.Writer) (core.CapabilityResult, error)
}

func awsDownloadCommand(ctx context.Context, client awsClient, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("aws s3 cp", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	var spec awsplugin.GetSpec
	flags.StringVar(&spec.Environment, "env", "", cliMessage("detail.aws_env"))
	flags.StringVar(&spec.Profile, "profile", "default", cliMessage("detail.aws_profile"))
	flags.StringVar(&spec.Region, "region", "", cliMessage("detail.aws_region"))
	if flags.Parse(args) != nil || len(flags.Args()) != 2 {
		fmt.Fprintln(diagnostic, "Usage: haco aws s3 cp [--env name] [--profile name] [--region region] s3://bucket/key <file>")
		return 2
	}
	spec.URL = flags.Args()[0]
	if err := selectAWSEnvironment(ctx, client, &spec.Environment); err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 2
	}

	download, ok := client.(awsDownloadClient)
	if !ok {
		fmt.Fprintln(diagnostic, "haco: AWS downloads are unavailable")
		return 1
	}
	fmt.Fprintln(diagnostic, "If approval is required, review the notification or use haco approve. The destination is published only after verified completion.")
	if err := saveAWSDownload(ctx, download, spec, flags.Args()[1]); err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	fmt.Fprintln(out, "Downloaded and verified.")
	return 0
}
