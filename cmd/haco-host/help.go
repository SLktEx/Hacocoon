package main

import (
	"io"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

var hostHelpPages = []cliui.CommandHelp{
	{Path: "", Syntax: "<command>", Message: "command.host", Example: "haco-host env list"},
	{Path: "env", Syntax: "<command>", Message: "command.env", Example: "haco-host env list"},
	{Path: "env list", Syntax: "[--json]", Message: "command.env.list", Example: "haco-host env list"},
	{Path: "env create", Syntax: "--workspace <path> [--read-only] [--base <base>] [--cpu <n|unlimited>] [--memory <size|unlimited>] [--pids <n|unlimited>] [--root-size <size|unlimited>] <environment>", Message: "command.env.create", Example: "haco-host env create --workspace /work/project dev"},
	{Path: "env status", Syntax: "<environment> [--json]", Message: "command.env.status", Example: "haco-host env status dev"},
	{Path: "env exec", Syntax: "<environment> -- <command...>", Message: "command.env.exec", Example: "haco-host env exec dev -- pwd"},
	{Path: "env shell", Syntax: "<environment>", Message: "command.env.shell", Example: "haco-host env shell dev"},
	{Path: "env delete", Syntax: "<environment>", Message: "command.env.delete", Example: "haco-host env delete dev"},
	{Path: "doctor", Syntax: "", Message: "command.host.doctor", Example: "haco-host doctor"},
}

func writeHostHelp(out io.Writer, path string) bool {
	return cliui.WriteCommandHelp(out, "haco-host", path, hostHelpPages, cliui.Resolve(os.Getenv))
}

func requestedHostHelp(args []string, out io.Writer) bool {
	if len(args) == 1 && args[0] == "help" {
		return writeHostHelp(out, "")
	}
	if len(args) == 0 || (args[len(args)-1] != "--help" && args[len(args)-1] != "-h") {
		return false
	}
	return writeHostHelp(out, strings.Join(args[:len(args)-1], " "))
}
