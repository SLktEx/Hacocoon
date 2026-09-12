//go:build linux

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/sshclient"
)

func runSSH(args []string) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(os.Stdout, "Usage: haco ssh setup [environment]; blank selection cancels. Keys stay on the desktop client.")
		return 0
	}
	if len(args) == 0 || args[0] != "setup" || len(args) > 2 {
		fmt.Fprintln(os.Stderr, "Usage: haco ssh setup [environment]")
		return 2
	}
	return setupDesktopSSH(args[1:], "")
}
func runOpen(args []string) int {
	flags := flag.NewFlagSet("haco open", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: haco open [--client vscode|ssh | --port <port> [--close | --no-browser]] [environment]\nOpens an existing Env at /workspace; path discovery is not implemented. Blank selection cancels.")
		flags.PrintDefaults()
	}
	port := flags.Int("port", 0, "open an Environment HTTP port in the browser")
	closePreview := flags.Bool("close", false, "close the selected preview port")
	noBrowser := flags.Bool("no-browser", false, "print the preview URL without launching a browser")
	selected := flags.String("client", "vscode", "desktop client: vscode or ssh")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 1 || (*selected != "vscode" && *selected != "ssh") {
		fmt.Fprintln(os.Stderr, "Usage: haco open [--client vscode|ssh | --port <port> [--close | --no-browser]] [environment]")
		return 2
	}
	portSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			portSet = true
		}
	})
	if portSet {
		if *port < 1 || *port > 65535 || *selected != "vscode" {
			return 2
		}
		name := ""
		if flags.NArg() == 1 {
			name = flags.Arg(0)
		}
		return openPreview(name, *port, *closePreview, *noBrowser, os.Stdout, os.Stderr)
	}
	if *closePreview || *noBrowser {
		fmt.Fprintln(os.Stderr, "haco: --close and --no-browser require --port")
		return 2
	}
	return setupDesktopSSH(flags.Args(), *selected)
}
func setupDesktopSSH(args []string, launch string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	name := ""
	var selectedEnvironment *core.Environment
	if len(args) == 1 {
		name = args[0]
	} else {
		envs, err := client.ListEnvironments(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 1
		}
		chosen, err := chooseDesktopEnvironment(envs, stdioIsInteractive(), os.Stdin, os.Stderr)
		if errors.Is(err, errEnvironmentChoiceCanceled) {
			return 0
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 2
		}
		name = chosen.Name
		selectedEnvironment = &chosen
	}
	fmt.Fprintln(os.Stderr, "[running] desktop_client")
	desktop, err := sshclient.ResolveDesktop(ctx)
	if err != nil {
		return dailyFailure(os.Stderr, "open", "desktop_client", name, err)
	}
	fmt.Fprintln(os.Stderr, "[succeeded] desktop_client\n[running] ssh_connection")
	var alias string
	if selectedEnvironment != nil {
		alias, err = sshclient.SetupSelected(ctx, client, desktop, *selectedEnvironment)
	} else {
		alias, err = sshclient.Setup(ctx, client, desktop, name)
	}
	if err != nil {
		return dailyFailure(os.Stderr, "open", "ssh_connection", name, err)
	}
	fmt.Fprintln(os.Stderr, "[succeeded] ssh_connection")
	fmt.Fprintln(os.Stdout, "SSH ready:", alias)
	if launch == "" {
		return 0
	}
	if launch == "ssh" {
		executable := "ssh"
		if desktop.Windows {
			executable = "ssh.exe"
		}
		command := exec.Command(executable, "-t", alias, "cd /workspace && exec bash -l")
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "haco: SSH client:", err)
			return 1
		}
		return 0
	}
	executable, err := sshclient.Editor(ctx, desktop)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[failed] operation=open stage=editor_launch reason=unavailable\nSSH preparation completed; the Env and connection remain. Use haco open --client ssh <name>, or install/configure the desktop editor.")
		return 1
	}
	// A desktop process outlives this CLI. Its stdio must not retain the
	// controller/Incus command streams and prevent the invoking shell returning.
	command := exec.Command(executable, "--folder-uri", "vscode-remote://ssh-remote+"+alias+"/workspace")
	fmt.Fprintln(os.Stderr, "[running] editor_launch")
	if err = command.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "[failed] operation=open stage=editor_launch reason=failed; SSH remains prepared")
		return 1
	}
	if err = command.Process.Release(); err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, "Editor process launched; connection/edit/build/test readiness is not yet confirmed.")
	return 0
}
