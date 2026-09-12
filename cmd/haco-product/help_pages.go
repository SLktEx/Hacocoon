package main

import (
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/cliui"
	"io"
	"strings"
)

// Human-facing command metadata. Dispatch and authorization remain in their owners.
type helpPage struct{ path, syntax, message, example string }

var helpPages = []helpPage{
	{"env", "<command>", "command.env", "haco env list"},
	{"env list", "[--json]", "command.env.list", "haco env list"},
	{"env create", "--workspace <workspace> [--base <base>] [--resource oci:<store>] [--no-oci] <name>", "command.env.create", "haco env create --workspace managed:work dev"},
	{"env status", "[--json] <name>", "command.env.status", "haco env status dev"},
	{"env start", "<name>", "command.env.start", "haco env start dev"},
	{"env stop", "<name>", "command.env.stop", "haco env stop dev"},
	{"env delete", "<name>", "command.env.delete", "haco env delete dev"},
	{"env ssh", "--key <public-key-file> [--port <port>] <name>", "command.env.ssh", "haco env ssh --key /path/to/key.pub dev"},
	{"env ssh-config", "<name>", "command.env.ssh-config", "haco env ssh-config dev"},
	{"env disconnect", "<name> <connection-id>", "command.env.disconnect", "haco env disconnect dev <connection-id>"},
	{"env forward", "--target-port <port> [--protocol tcp|udp] [--port <local-port>] <name>", "command.env.forward", "haco env forward --target-port 8080 dev"},
	{"env copy", "[--json] <stopped-env> [new-env]", "command.env.copy", "haco env copy dev dev-copy"},
	{"env export", "[--json] <stopped-env> [file.haco]", "command.env.export", "haco env export dev dev.haco"},
	{"env import", "[--json] <file.haco> [new-env]", "command.env.import", "haco env import dev.haco restored"},
	{"repo", "<command>", "command.repo", "haco repo list"},
	{"repo clone", "--branch <branch> <id> <URL>", "command.repo.clone", "haco repo clone --branch main source https://github.com/OWNER/REPO.git"},
	{"repo list", "[--json]", "command.repo.list", "haco repo list"},
	{"repo delete", "[--yes] <id>", "command.repo.delete", "haco repo delete source"},
	{"workspace", "<command>", "command.workspace", "haco workspace list"},
	{"workspace create", "--repo <id[,id...]> <workspace>", "command.workspace.create", "haco workspace create --repo source work"},
	{"workspace prepare", "--path <directory> --repo <id[,id...]> [--name <name>] [--base <base>] [--oci auto|none|oci:<store>]", "command.workspace.prepare", "haco workspace prepare --path . --repo source --name work"},
	{"workspace fork", "--path <directory> [--name <name>] [--base <base>] <source>", "command.workspace.fork", "haco workspace fork --path ../branch --name branch work"},
	{"workspace list", "[--json]", "command.workspace.list", "haco workspace list"},
	{"workspace delete", "[--yes] <workspace>", "command.workspace.delete", "haco workspace delete work"},
	{"git", "<command>", "command.git", "haco git pending"},
	{"git connect", "<environment>", "command.git.connect", "haco git connect dev"},
	{"git pending", "", "command.git.pending", "haco git pending"},
	{"git approve", "[--save env|all|ask-env|ask-all] <id>", "command.git.approve", "haco git approve <id>"},
	{"git deny", "[--save env|all|ask-env|ask-all] <id>", "command.git.deny", "haco git deny <id>"},
	{"base", "<command>", "command.base", "haco base list"},
	{"base list", "[--all [--json]]", "command.base.list", "haco base list"},
	{"base inspect", "<base>", "command.base.inspect", "haco base inspect <base>"},
	{"base build", "<definition.json>", "command.base.build", "haco base build definition.json"},
	{"base delete", "[--yes] <name-or-fingerprint>", "command.base.delete", "haco base delete <base>"},
	{"snapshot", "<command>", "command.snapshot", "haco snapshot list"},
	{"snapshot create", "[--json] <env>", "command.snapshot.create", "haco snapshot create dev"},
	{"snapshot list", "[--json] [env]", "command.snapshot.list", "haco snapshot list"},
	{"snapshot restore", "[--json] <snapshot-id> [new-env]", "command.snapshot.restore", "haco snapshot restore <snapshot-id> restored"},
	{"snapshot delete", "<snapshot-id>", "command.snapshot.delete", "haco snapshot delete <snapshot-id>"},
	{"plugin", "<command>", "command.plugin", "haco plugin oci --help"},
	{"plugin oci", "<command>", "command.plugin.oci", "haco plugin oci store list"},
	{"plugin oci store", "<command>", "command.plugin.oci.store", "haco plugin oci store list"},
	{"plugin oci store list", "[--json]", "command.plugin.oci.store.list", "haco plugin oci store list"},
	{"plugin oci store inspect", "<store>", "command.plugin.oci.store.inspect", "haco plugin oci store inspect <store>"},
	{"plugin oci store create", "<store> [--from <store>]", "command.plugin.oci.store.create", "haco plugin oci store create work"},
	{"plugin oci store delete", "[--yes] <store>", "command.plugin.oci.store.delete", "haco plugin oci store delete work"},
	{"plugin oci image", "<command>", "command.plugin.oci.image", "haco plugin oci image list <env>"},
	{"plugin oci image list", "[--unused] [--runtime nerdctl|docker] [--json] [--host] [<env-or-store-id>]", "command.plugin.oci.image.list", "haco plugin oci image list dev"},
	{"plugin oci image delete", "[--unused] [--runtime nerdctl|docker] [--yes] [--host] [<env-or-store-id>] [<image-id-or-tag>]", "command.plugin.oci.image.delete", "haco plugin oci image delete dev <image-id>"},
	{"network", "<command>", "command.network", "haco network list"},
	{"network tcp", "--target <name> --port <port> [--kind external|host|environment] [--listen 127.0.0.1:0] [--duration 5m]", "command.network.tcp", "haco network tcp --target example.com --port 443"},
	{"network udp", "--target <name> --port <port> [--kind external|host|environment] [--listen 127.0.0.1:0] [--duration 5m]", "command.network.udp", "haco network udp --target <host> --port 53"},
	{"network list", "", "command.network.list", "haco network list"},
	{"network revoke", "<connection-id>", "command.network.revoke", "haco network revoke <connection-id>"},
	{"network host", "<command>", "command.network.host", "haco network host list"},
	{"network host list", "", "command.network.host.list", "haco network host list"},
	{"network host add", "--address <IP> --port <port> [--protocol tcp|udp] <name>", "command.network.host.add", "haco network host add --address 192.0.2.1 --port 8080 service"},
	{"network host remove", "<name>", "command.network.host.remove", "haco network host remove service"},
	{"network rule", "--env <name> --target <name> --port <port> --decision allow|ask|deny [--kind external|host|environment] [--protocol tcp|udp] [--duration 5m] [--ttl 1h] [--scope instance|environment|global]", "command.network.rule", "haco network rule --env dev --target example.com --port 443 --decision ask"},
	{"aws", "<command>", "command.aws", "haco aws s3 --help"},
	{"aws s3", "<command>", "command.aws.s3", "haco aws s3 ls s3://bucket/prefix"},
	{"aws s3 ls", "[--env <name>] [--profile <name>] [--region <region>] <s3://bucket/prefix>", "command.aws.s3.ls", "haco aws s3 ls s3://bucket/prefix"},
	{"aws s3 cp", "[--env <name>] [--profile <name>] [--region <region>] <s3://bucket/key> <file>", "command.aws.s3.cp", "haco aws s3 cp s3://bucket/key download"},
	{"ssh", "<command>", "command.ssh", "haco ssh setup dev"},
	{"ssh setup", "[environment]", "command.ssh.setup", "haco ssh setup dev"},
	{"open", "[--client vscode|ssh|none] [--repo <id[,id...]>] [--name <name>] [--base <base>] [--oci auto|none|oci:<store>] [--port <port>] [--close] [--no-browser] [environment-or-directory]", "command.open", "haco open --repo source ."},
}

func commandHelp(out io.Writer, path string, language cliui.Language) bool {
	for _, page := range helpPages {
		if page.path != path {
			continue
		}
		fmt.Fprint(out, cliui.HelpLines("", language.Text(page.message), 60))
		fmt.Fprintln(out, language.Text("help.usage"))
		// Keep each option on a separate continuation line, independent of TTY width.
		syntax := strings.ReplaceAll(page.syntax, " [", "\n    [")
		fmt.Fprintf(out, "  haco %s %s\n", path, syntax)
		children := false
		for _, child := range helpPages {
			suffix, ok := strings.CutPrefix(child.path, path+" ")
			if !ok || strings.Contains(suffix, " ") {
				continue
			}
			if !children {
				fmt.Fprintln(out, "\n"+language.Text("help.commands"))
				children = true
			}
			fmt.Fprint(out, cliui.HelpLines(fmt.Sprintf("  %-14s", suffix), language.Text(child.message), 60))
		}
		if children {
			fmt.Fprintf(out, "\n  haco %s <command> --help\n", path)
		}
		fmt.Fprintln(out, "\n"+language.Text("help.example"))
		fmt.Fprintln(out, "  "+page.example)
		return true
	}
	return false
}

// Only literal command paths followed by help are intercepted. In particular,
// `haco run -- tool --help` remains an Execution, not Hacocoon help.
func requestedCommandHelp(args []string, out io.Writer) bool {
	if len(args) < 2 || (args[len(args)-1] != "--help" && args[len(args)-1] != "-h") {
		return false
	}
	return commandHelp(out, strings.Join(args[:len(args)-1], " "), cliLanguage())
}
