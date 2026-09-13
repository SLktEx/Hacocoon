package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type gitRecoveryClient interface {
	GitPushStatus(context.Context, controlapi.GitStatusRequest, bool) (gitrepo.PushStatus, error)
}

func gitRecoveryCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	return gitRecoveryWithClient(ctx, nil, args, out, diagnostic)
}

func gitRecoveryWithClient(ctx context.Context, client gitRecoveryClient, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 || (args[0] != "status" && args[0] != "reconcile") {
		return 2
	}
	flags := flag.NewFlagSet("git "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	jsonOutput := flags.Bool("json", false, cliMessage("flag.json"))
	requestID := flags.String("request", "", cliMessage("git.recovery.request"))
	if flags.Parse(args[1:]) != nil || flags.NArg() != 1 {
		commandHelp(diagnostic, "git "+args[0], cliLanguage())
		return 2
	}
	if client == nil {
		c, err := controlapi.NewDefaultClient()
		if err != nil {
			fmt.Fprintln(diagnostic, cliMessage("error.controller"))
			return 1
		}
		client = c
	}
	result, err := client.GitPushStatus(ctx, controlapi.GitStatusRequest{Environment: flags.Arg(0), RequestID: *requestID}, args[0] == "reconcile")
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("operation.failed"), err)
		return 1
	}
	if *jsonOutput {
		err = json.NewEncoder(out).Encode(result)
	} else {
		err = writeGitRecovery(out, result)
	}
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.write_result"))
		return 1
	}
	return 0
}

func writeGitRecovery(out io.Writer, result gitrepo.PushStatus) error {
	if !result.Found {
		_, err := fmt.Fprintln(out, cliMessage("git.recovery.none"))
		return err
	}
	if _, err := fmt.Fprintln(out, cliMessage("git.recovery.target", result.Environment, result.Repository, result.Ref)); err != nil {
		return err
	}
	state := "git.recovery.unconfirmed"
	if result.State == "confirmed" {
		state = "git.recovery.confirmed"
	}
	if _, err := fmt.Fprintln(out, cliMessage(state)); err != nil {
		return err
	}
	if result.Completed != nil && !*result.Completed {
		if _, err := fmt.Fprintln(out, cliMessage("git.recovery.failed")); err != nil {
			return err
		}
	}
	if result.Active {
		_, err := fmt.Fprintln(out, cliMessage("git.recovery.active"))
		return err
	}
	if result.Observation != "" {
		if _, err := fmt.Fprintln(out, cliMessage("git.recovery."+result.Observation)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, cliMessage("git.recovery.next"))
	return err
}
