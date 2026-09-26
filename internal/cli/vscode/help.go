package vscodecli

import (
	"io"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
)

func writeAdapterHelp(out io.Writer, path string, language cliui.Language) {
	name := cliui.HelpField{Syntax: "--name <name>", Message: "vscode.name"}
	workspace := []cliui.HelpField{{Syntax: "<workspace>", Message: "vscode.workspace"}}
	cliui.WriteCommandHelp(out, "haco-vscode", path, []cliui.CommandHelp{
		{Path: "", Syntax: "<command>", Message: "vscode.help", Example: "haco-vscode open --help"},
		{Path: "open", Syntax: "[--name <name>] [--read-only] [--no-launch] <workspace>", Message: "vscode.open", Arguments: workspace,
			Options: []cliui.HelpField{name, {Syntax: "--read-only", Message: "vscode.read_only"}, {Syntax: "--no-launch", Message: "vscode.no_launch"}}, Example: "haco-vscode open --name demo /work/project"},
		{Path: "delete", Syntax: "[--name <name>] <workspace>", Message: "vscode.delete", Arguments: workspace,
			Options: []cliui.HelpField{name}, Example: "haco-vscode delete --name demo /work/project"},
	}, language)
}
