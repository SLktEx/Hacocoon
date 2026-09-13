package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func runRepository(namespace string, args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return repositoryCommand(ctx, namespace, args, os.Stdout, os.Stderr)
}
func repositoryCommand(ctx context.Context, namespace string, args []string, out, diagnostic io.Writer) int {
	if requestedCommandHelp(append([]string{namespace}, args...), out) {
		return 0
	}
	if namespace == "workspace" && len(args) > 0 && (args[0] == "prepare" || args[0] == "fork") {
		return workflowCommand(ctx, args, out, diagnostic)
	}
	if namespace == "git" && len(args) > 0 && (args[0] == "status" || args[0] == "reconcile") {
		return gitRecoveryCommand(ctx, args, out, diagnostic)
	}
	usage := func() int {
		commandHelp(diagnostic, namespace, cliLanguage())
		return 2
	}
	if namespace == "repo" && len(args) > 0 && (args[0] == "list" || args[0] == "delete") {
		c, err := controlapi.NewDefaultClient()
		if err != nil {
			fmt.Fprintln(diagnostic, cliMessage("operation.failed"), err)
			return 1
		}
		return sourceManageCommand(ctx, c, args, os.Stdin, out, diagnostic)
	}
	if namespace == "workspace" && len(args) > 0 && (args[0] == "list" || args[0] == "delete") {
		return managedWorkspaceCommand(ctx, args, os.Stdin, out, diagnostic)
	}
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(diagnostic, "haco:", flagErr)
		return 2
	}
	args = clean
	if len(args) == 0 {
		return usage()
	}
	if args[0] == "--help" || args[0] == "-h" {
		usage()
		return 0
	}
	operation := namespace + " " + args[0]
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	var branch, repo, save string
	n := 1
	switch operation {
	case "repo clone":
		flags.StringVar(&branch, "branch", "", cliMessage("detail.branch"))
		n = 2
	case "workspace create":
		flags.StringVar(&repo, "repo", "", cliMessage("detail.repos"))
	case "git connect":
	case "git approve", "git deny":
		flags.StringVar(&save, "save", "", cliMessage("detail.saved"))
	case "git pending":
		n = 0
	default:
		return usage()
	}
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	pos := flags.Args()
	if len(pos) != n || (operation == "repo clone" && branch == "") || (operation == "workspace create" && repo == "") {
		return usage()
	}
	var choice capabilityapp.SavedChoice
	if save != "" {
		switch save {
		case "env":
			choice = capabilityapp.DenyEnvironment
			if operation == "git approve" {
				choice = capabilityapp.AllowEnvironment
			}
		case "all":
			choice = capabilityapp.DenyGlobal
			if operation == "git approve" {
				choice = capabilityapp.AllowGlobal
			}
		case "ask-env":
			choice = capabilityapp.AskEnvironment
		case "ask-all":
			choice = capabilityapp.AskGlobal
		default:
			return usage()
		}
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.controller"))
		return 1
	}
	var result any
	switch operation {
	case "repo clone":
		result, err = client.CloneRepository(ctx, controlapi.RepositoryCloneRequest{ID: pos[0], Remote: pos[1], Branch: branch})
	case "workspace create":
		request := controlapi.WorkspaceCopyRequest{ID: pos[0], Repository: repo}
		if strings.Contains(repo, ",") {
			request.Repository = ""
			request.Repositories = strings.Split(repo, ",")
		}
		result, err = client.CopyWorkspace(ctx, request)
	case "git connect":
		err = client.ConnectGit(ctx, pos[0])
		result = "Environment Git helper connected"
	case "git pending":
		result, err = client.PendingGit(ctx)
	case "git approve", "git deny":
		approved := operation == "git approve"
		if choice == "" {
			err = client.DecideGit(ctx, pos[0], approved)
			result = "Decision recorded for the displayed fixed request"
		} else {
			result, err = client.DecideGitWithSavedChoice(ctx, pos[0], approved, choice)
		}
	}
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: %v\n", err)
		return 1
	}
	if err := writeCLIResult(out, result, jsonOutput); err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.write_result"))
		return 1
	}
	return 0
}
