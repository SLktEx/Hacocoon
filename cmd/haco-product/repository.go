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
)

func runRepository(namespace string, args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return repositoryCommand(ctx, namespace, args, os.Stdout, os.Stderr)
}
func repositoryCommand(ctx context.Context, namespace string, args []string, out, diagnostic io.Writer) int {
	usage := func() int {
		fmt.Fprintln(diagnostic, "Usage: haco repo clone --branch <branch> <id> <URL> | haco workspace create --repo <id> <workspace> | haco git connect <environment> | haco git pending | haco git approve <id> | haco git deny <id>")
		return 2
	}
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
	var branch, repo string
	n := 1
	switch operation {
	case "repo clone":
		flags.StringVar(&branch, "branch", "", "one existing upstream branch")
		n = 2
	case "workspace create":
		flags.StringVar(&repo, "repo", "", "registered repository")
	case "git connect", "git approve", "git deny":
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
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot open controller client")
		return 1
	}
	var result any
	switch operation {
	case "repo clone":
		result, err = client.CloneRepository(ctx, controlapi.RepositoryCloneRequest{ID: pos[0], Remote: pos[1], Branch: branch})
	case "workspace create":
		result, err = client.CopyWorkspace(ctx, controlapi.WorkspaceCopyRequest{ID: pos[0], Repository: repo})
	case "git connect":
		err = client.ConnectGit(ctx, pos[0])
		result = "Environment Git helper connected"
	case "git pending":
		result, err = client.PendingGit(ctx)
	case "git approve":
		err = client.DecideGit(ctx, pos[0], true)
		result = "Approval recorded for the displayed fixed request"
	case "git deny":
		err = client.DecideGit(ctx, pos[0], false)
		result = "Push denied"
	}
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot write result")
		return 1
	}
	return 0
}
