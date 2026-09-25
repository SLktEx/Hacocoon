package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/policy"
	"golang.org/x/term"
)

func runRepository(namespace string, args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	operationArgs, _, _ := splitJSONFlag(args)
	if namespace != "repo" || len(operationArgs) == 0 || operationArgs[0] != "add" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
	}
	return repositoryCommand(ctx, namespace, args, os.Stdout, os.Stderr)
}
func repositoryCommand(ctx context.Context, namespace string, args []string, out, diagnostic io.Writer) int {
	if requestedCommandHelp(append([]string{namespace}, args...), out) {
		return 0
	}
	if namespace == "workspace" && len(args) > 0 && (args[0] == "prepare" || args[0] == "fork" || args[0] == "import") {
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
		c := controlapi.NewDefaultClient()
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
	case "repo add":
		n = 2
	case "workspace create":
		flags.StringVar(&branch, "branch", "", cliMessage("detail.branch"))
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
	if len(pos) != n || (operation == "workspace create" && (repo == "" || (branch != "" && strings.Contains(repo, ",")))) {
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
	client := controlapi.NewDefaultClient()
	var result any
	var err error
	switch operation {
	case "repo add":
		progress := newRepositoryProgressWriter(diagnostic)
		result, err = client.AddRepository(ctx, controlapi.RepositoryAddRequest{ID: pos[0], Remote: pos[1]}, progress)
		if finishErr := progress.Finish(); err == nil && finishErr != nil {
			err = finishErr
		}
	case "workspace create":
		request := controlapi.WorkspaceCopyRequest{ID: pos[0], Repository: repo, Branch: branch}
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
		_, _ = fmt.Fprintln(diagnostic, cliMessage("error.write_result"))
		return 1
	}
	return 0
}


type repositoryProgressWriter struct {
	out      io.Writer
	terminal bool
	active   bool
	stage    string
	pending  []byte
}

func newRepositoryProgressWriter(out io.Writer) *repositoryProgressWriter {
	terminalOutput := false
	if file, ok := out.(interface{ Fd() uintptr }); ok {
		terminalOutput = term.IsTerminal(int(file.Fd()))
	}
	return &repositoryProgressWriter{out: out, terminal: terminalOutput}
}

func (w *repositoryProgressWriter) Write(p []byte) (int, error) {
	if !w.terminal {
		return w.out.Write(p)
	}
	w.pending = append(w.pending, p...)
	for {
		end := bytes.IndexByte(w.pending, '\n')
		if end < 0 {
			return len(p), nil
		}
		line := strings.TrimSuffix(string(w.pending[:end]), "\r")
		w.pending = w.pending[end+1:]
		if err := w.render(line); err != nil {
			return len(p), err
		}
	}
}

func (w *repositoryProgressWriter) Finish() error {
	if !w.terminal {
		return nil
	}
	if len(w.pending) != 0 {
		line := strings.TrimSuffix(string(w.pending), "\r")
		w.pending = nil
		if err := w.render(line); err != nil {
			return err
		}
	}
	if w.active {
		w.active = false
		_, err := io.WriteString(w.out, "\n")
		return err
	}
	return nil
}

func (w *repositoryProgressWriter) render(line string) error {
	if line == "" {
		return nil
	}
	stage := line
	if prefix, _, ok := strings.Cut(line, ":"); ok {
		stage = prefix
	}
	if w.active && stage != w.stage {
		if _, err := io.WriteString(w.out, "\n"); err != nil {
			return err
		}
		w.active = false
	}
	if _, err := io.WriteString(w.out, "\r\x1b[2K"+line); err != nil {
		return err
	}
	w.active = true
	w.stage = stage
	return nil
}
