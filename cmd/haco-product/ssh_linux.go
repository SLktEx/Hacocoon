//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/sshclient"
)

func runSSH(args []string) int {
	if len(args) == 0 || args[0] != "setup" || len(args) > 2 {
		fmt.Fprintln(os.Stderr, "Usage: haco ssh setup [environment]")
		return 2
	}
	return setupDesktopSSH(args[1:], false)
}
func runOpen(args []string) int {
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "Usage: haco open [environment]")
		return 2
	}
	return setupDesktopSSH(args, true)
}
func setupDesktopSSH(args []string, launch bool) int {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	name := ""
	if len(args) == 1 {
		name = args[0]
	} else {
		envs, err := client.ListEnvironments(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 1
		}
		if len(envs) != 1 {
			fmt.Fprintln(os.Stderr, "haco: select an Environment by name; available Environments:")
			for _, env := range envs {
				fmt.Fprintln(os.Stderr, " ", env.Name)
			}
			return 2
		}
		name = envs[0].Name
	}
	desktop, err := sshclient.ResolveDesktop(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	alias, err := sshclient.Setup(ctx, client, desktop, name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "SSH ready:", alias)
	if !launch {
		return 0
	}
	executable, err := sshclient.Editor(ctx, desktop)
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: SSH is prepared; cannot launch VS Code:", err)
		return 1
	}
	command := exec.Command(executable, "--folder-uri", "vscode-remote://ssh-remote+"+alias+"/workspace")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err = command.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "haco: launch VS Code:", err)
		return 1
	}
	if err = command.Process.Release(); err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	return 0
}
