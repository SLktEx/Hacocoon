//go:build linux

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func runOpen(args []string) int {
	// Keep the existing explicit directory workflow available as an advanced entry.
	fresh := false
	for _, arg := range args {
		if arg == "--new" || strings.HasPrefix(arg, "--new=") {
			fresh = true
		}
	}
	if !fresh {
		for _, arg := range args {
			if workspacePath(arg) || arg == "--select" {
				return openWorkspaceDirectory(args)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	client := controlapi.NewDefaultClient()
	return openCreationCommand(ctx, client, args, os.Stdout, os.Stderr, func(env core.Environment, selected string) int {
		if strings.HasPrefix(env.Workspace.Path, "managed:") {
			if err := client.ConnectGit(ctx, env.Name); err != nil && !isUnsupportedControllerError(err) {
				fmt.Fprintln(os.Stderr, "haco:", err)
				return 1
			}
		}
		if selected == "none" {
			return 0
		}
		return setupDesktopSSHSelected([]string{env.Name}, selected, &env)
	})
}

func openCreationCommand(ctx context.Context, client creationClient, args []string, out, diagnostic io.Writer, launch func(core.Environment, string) int) int {
	flags := flag.NewFlagSet("haco open", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	fresh := flags.Bool("new", false, "Create a new Environment")
	name := flags.String("name", "", "Name for the new Environment")
	volume := flags.String("volume", "", "Workspace Volume for the new Environment")
	snapshot := flags.String("snapshot", "", "Create from a Snapshot")
	selected := flags.String("client", "vscode", "vscode, ssh, or none")
	machine := flags.Bool("json", false, cliMessage("flag.json"))
	port := flags.Int("port", 0, cliMessage("detail.preview_port"))
	closePreview := flags.Bool("close", false, cliMessage("detail.close_preview"))
	noBrowser := flags.Bool("no-browser", false, cliMessage("detail.no_browser"))
	if err := parseInterspersed(flags, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			commandHelp(out, "open", cliLanguage())
			return 0
		}
		return 2
	}
	portSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			portSet = true
		}
	})
	if (portSet && *port == 0) || flags.NArg() > 1 || (*selected != "vscode" && *selected != "ssh" && *selected != "none") || (*machine && *selected != "none") ||
		(!*fresh && (*name != "" || *volume != "" || *snapshot != "")) ||
		(*snapshot != "" && (*volume != "" || flags.NArg() != 0)) ||
		((*closePreview || *noBrowser) && *port == 0) || *port < 0 || *port > 65535 || (*port != 0 && (*fresh || *selected != "vscode")) {
		commandHelp(diagnostic, "open", cliLanguage())
		return 2
	}
	if *port != 0 {
		return openPreview(flags.Arg(0), *port, *closePreview, *noBrowser, out, diagnostic)
	}
	request := controlapi.OpenRequest{Environment: flags.Arg(0)}
	if *fresh {
		request.Environment = ""
		request.New = &creation.Request{Name: *name, Image: core.BaseName(flags.Arg(0)), Volume: *volume, Snapshot: *snapshot}
	}
	var environment core.Environment
	err := openStage(ctx, diagnostic, "environment", func() error {
		var err error
		environment, err = client.OpenTarget(ctx, request)
		return err
	})
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	if *machine {
		if writeCLIResult(out, environment, true) != nil {
			return 1
		}
	} else {
		if _, err := fmt.Fprintf(out, "Environment %q ready.\n", environment.Name); err != nil {
			return 1
		}
	}
	return launch(environment, *selected)
}
