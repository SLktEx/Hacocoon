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
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
)

type awsClient interface {
	ListEnvironments(context.Context) ([]core.Environment, error)
	ListS3(context.Context, awsplugin.ListSpec) (core.CapabilityResult, error)
}

func runAWS(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	var client awsClient
	var err error
	executable, executableErr := os.Executable()
	if executableErr == nil && executable == "/usr/local/libexec/hacocoon-dns" {
		client = awsplugin.NewGuestClient()
	} else {
		client, err = controlapi.NewDefaultClient()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: cannot open controller client")
		return 1
	}
	return awsCommand(ctx, client, args, os.Stdout, os.Stderr)
}
func awsCommand(ctx context.Context, client awsClient, args []string, out, diagnostic io.Writer) int {
	if len(args) >= 2 && args[0] == "s3" && args[1] == "cp" {
		return awsDownloadCommand(ctx, client, args[2:], out, diagnostic)
	}
	usage := func() int {
		fmt.Fprintln(diagnostic, "Usage: haco aws s3 ls [--env name] [--profile name] [--region region] s3://bucket/prefix")
		fmt.Fprintln(diagnostic, "       haco aws s3 cp [--env name] [--profile name] [--region region] s3://bucket/key <file>")
		return 2
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		usage()
		return 0
	}
	if len(args) < 2 || args[0] != "s3" || args[1] != "ls" {
		return usage()
	}
	flags := flag.NewFlagSet("aws s3 ls", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	var spec awsplugin.ListSpec
	flags.StringVar(&spec.Environment, "env", "", "Environment (inferred when only one exists)")
	flags.StringVar(&spec.Profile, "profile", "default", "trusted Host AWS profile")
	flags.StringVar(&spec.Region, "region", "", "AWS region (defaults to trusted Host profile)")
	if flags.Parse(args[2:]) != nil || len(flags.Args()) != 1 {
		return usage()
	}
	spec.URL = flags.Args()[0]
	if err := selectAWSEnvironment(ctx, client, &spec.Environment); err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 2
	}

	fmt.Fprintln(diagnostic, "AWS authentication stays in Host. If approval is required, review the notification or use haco approve.")
	result, err := client.ListS3(ctx, spec)
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: %v (request %s; execution %s)\n", err, result.RequestID, result.ExecutionState)
		return 1
	}
	if result.ExecutionState != core.CapabilitySucceeded || !result.AuditComplete || !json.Valid([]byte(result.Output)) {
		fmt.Fprintln(diagnostic, "haco: incomplete AWS result")
		return 1
	}
	if _, err := fmt.Fprintln(out, result.Output); err != nil {
		return 1
	}
	return 0
}

func selectAWSEnvironment(ctx context.Context, client awsClient, name *string) error {
	if guest, ok := client.(interface{ GuestSource() bool }); ok && guest.GuestSource() {
		if *name != "" {
			return fmt.Errorf("--env is unavailable inside an Environment; its source is selected automatically")
		}
		return nil
	}
	if *name != "" {
		return nil
	}
	envs, err := client.ListEnvironments(ctx)
	if err != nil || len(envs) != 1 {
		return fmt.Errorf("select an Environment with --env")
	}
	*name = envs[0].Name
	return nil
}
