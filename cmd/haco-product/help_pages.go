package main

import (
	"github.com/SLktEx/Hacocoon/internal/cliui"
	"io"
	"strings"
)

// Human-facing command metadata. Dispatch and authorization remain in their owners.
type helpPage = cliui.CommandHelp

var helpPages = []helpPage{
	{Path: "setup", Syntax: "[--script <path> | --clear-script] [environment]", Message: "help.setup", Example: "haco setup dev"},
	{Path: "config", Syntax: "[--edit | --file <file>]", Message: "help.config", Example: "haco config"},
	{Path: "approve", Syntax: "[--list] [--json] [request-id]", Message: "help.approve", Example: "haco approve --list"},
	{Path: "doctor", Syntax: "[--json] [environment]", Message: "help.doctor", Example: "haco doctor dev"},
	{Path: "reclaim", Syntax: "[--yes | --status | --review [--yes]]", Message: "help.reclaim", Example: "haco reclaim --status"},
	{Path: "version", Syntax: "[--json]", Message: "help.version", Example: "haco version --json"},
	{Path: "run", Syntax: "[-i | -it] [--workspace <workspace>] [--base <base>] [--no-oci] [--read-only] [--json] [--rm] -- <command...>", Message: "run.help", Example: "haco run -it -- bash"},
	{Path: "env", Syntax: "<command>", Message: "command.env", Example: "haco env list"},
	{Path: "env list", Syntax: "[--json]", Message: "command.env.list", Example: "haco env list"},
	{Path: "env create", Syntax: "--workspace <workspace> [--base <base>] [--resource oci:<store>] [--no-oci] <name>", Message: "command.env.create", Example: "haco env create --workspace managed:work dev"},
	{Path: "env status", Syntax: "[--json] <name>", Message: "command.env.status", Example: "haco env status dev"},
	{Path: "env start", Syntax: "<name>", Message: "command.env.start", Example: "haco env start dev"},
	{Path: "env stop", Syntax: "<name>", Message: "command.env.stop", Example: "haco env stop dev"},
	{Path: "env delete", Syntax: "<name>", Message: "command.env.delete", Example: "haco env delete dev"},
	{Path: "env ssh", Syntax: "--key <public-key-file> [--port <port>] <name>", Message: "command.env.ssh", Example: "haco env ssh --key /path/to/key.pub dev"},
	{Path: "env ssh-config", Syntax: "<name>", Message: "command.env.ssh-config", Example: "haco env ssh-config dev"},
	{Path: "env disconnect", Syntax: "<name> <connection-id>", Message: "command.env.disconnect", Example: "haco env disconnect dev <connection-id>"},
	{Path: "env forward", Syntax: "--target-port <port> [--protocol tcp|udp] [--port <local-port>] <name>", Message: "command.env.forward", Example: "haco env forward --target-port 8080 dev"},
	{Path: "env copy", Syntax: "[--json] <stopped-env> [new-env]", Message: "command.env.copy", Example: "haco env copy dev dev-copy"},
	{Path: "env export", Syntax: "[--json] <stopped-env> [file.haco]", Message: "command.env.export", Example: "haco env export dev dev.haco"},
	{Path: "env import", Syntax: "[--json] <file.haco> [new-env]", Message: "command.env.import", Example: "haco env import dev.haco restored"},
	{Path: "repo", Syntax: "<command>", Message: "command.repo", Example: "haco repo list"},
	{Path: "repo clone", Syntax: "--branch <branch> <id> <URL>", Message: "command.repo.clone", Example: "haco repo clone --branch main source https://github.com/OWNER/REPO.git"},
	{Path: "repo list", Syntax: "[--json]", Message: "command.repo.list", Example: "haco repo list"},
	{Path: "repo delete", Syntax: "[--yes] <id>", Message: "command.repo.delete", Example: "haco repo delete source"},
	{Path: "workspace", Syntax: "<command>", Message: "command.workspace", Example: "haco workspace list"},
	{Path: "workspace create", Syntax: "--repo <id[,id...]> <workspace>", Message: "command.workspace.create", Example: "haco workspace create --repo source work"},
	{Path: "workspace prepare", Syntax: "--path <directory> --repo <id[,id...]> [--name <name>] [--base <base>] [--oci auto|none|oci:<store>]", Message: "command.workspace.prepare", Example: "haco workspace prepare --path . --repo source --name work"},
	{Path: "workspace fork", Syntax: "--path <directory> [--name <name>] [--base <base>] <source>", Message: "command.workspace.fork", Example: "haco workspace fork --path ../branch --name branch work"},
	{Path: "workspace list", Syntax: "[--json]", Message: "command.workspace.list", Example: "haco workspace list"},
	{Path: "workspace delete", Syntax: "[--yes] <workspace>", Message: "command.workspace.delete", Example: "haco workspace delete work"},
	{Path: "git", Syntax: "<command>", Message: "command.git", Example: "haco git pending"},
	{Path: "git connect", Syntax: "<environment>", Message: "command.git.connect", Example: "haco git connect dev"},
	{Path: "git pending", Syntax: "", Message: "command.git.pending", Example: "haco git pending"},
	{Path: "git approve", Syntax: "[--save env|all|ask-env|ask-all] <id>", Message: "command.git.approve", Example: "haco git approve <id>"},
	{Path: "git deny", Syntax: "[--save env|all|ask-env|ask-all] <id>", Message: "command.git.deny", Example: "haco git deny <id>"},
	{Path: "base", Syntax: "<command>", Message: "command.base", Example: "haco base list"},
	{Path: "base list", Syntax: "[--all [--json]]", Message: "command.base.list", Example: "haco base list"},
	{Path: "base inspect", Syntax: "<base>", Message: "command.base.inspect", Example: "haco base inspect <base>"},
	{Path: "base build", Syntax: "<definition.json>", Message: "command.base.build", Example: "haco base build definition.json"},
	{Path: "base delete", Syntax: "[--yes] <name-or-fingerprint>", Message: "command.base.delete", Example: "haco base delete <base>"},
	{Path: "snapshot", Syntax: "<command>", Message: "command.snapshot", Example: "haco snapshot list"},
	{Path: "snapshot create", Syntax: "[--json] <env>", Message: "command.snapshot.create", Example: "haco snapshot create dev"},
	{Path: "snapshot list", Syntax: "[--json] [env]", Message: "command.snapshot.list", Example: "haco snapshot list"},
	{Path: "snapshot restore", Syntax: "[--json] <snapshot-id> [new-env]", Message: "command.snapshot.restore", Example: "haco snapshot restore <snapshot-id> restored"},
	{Path: "snapshot delete", Syntax: "<snapshot-id>", Message: "command.snapshot.delete", Example: "haco snapshot delete <snapshot-id>"},
	{Path: "plugin", Syntax: "<command>", Message: "command.plugin", Example: "haco plugin oci --help"},
	{Path: "plugin oci", Syntax: "<command>", Message: "command.plugin.oci", Example: "haco plugin oci store list"},
	{Path: "plugin oci store", Syntax: "<command>", Message: "command.plugin.oci.store", Example: "haco plugin oci store list"},
	{Path: "plugin oci store list", Syntax: "[--json]", Message: "command.plugin.oci.store.list", Example: "haco plugin oci store list"},
	{Path: "plugin oci store inspect", Syntax: "<store>", Message: "command.plugin.oci.store.inspect", Example: "haco plugin oci store inspect <store>"},
	{Path: "plugin oci store create", Syntax: "<store> [--from <store>]", Message: "command.plugin.oci.store.create", Example: "haco plugin oci store create work"},
	{Path: "plugin oci store delete", Syntax: "[--yes] <store>", Message: "command.plugin.oci.store.delete", Example: "haco plugin oci store delete work"},
	{Path: "plugin oci image", Syntax: "<command>", Message: "command.plugin.oci.image", Example: "haco plugin oci image list <env>"},
	{Path: "plugin oci image list", Syntax: "[--unused] [--runtime nerdctl|docker] [--json] [--host] [<env-or-store-id>]", Message: "command.plugin.oci.image.list", Example: "haco plugin oci image list dev"},
	{Path: "plugin oci image delete", Syntax: "[--unused] [--runtime nerdctl|docker] [--yes] [--host] [<env-or-store-id>] [<image-id-or-tag>]", Message: "command.plugin.oci.image.delete", Example: "haco plugin oci image delete dev <image-id>"},
	{Path: "network", Syntax: "<command>", Message: "command.network", Example: "haco network list"},
	{Path: "network tcp", Syntax: "--target <name> [--port <port>] [--kind external|host|environment] [--listen 127.0.0.1:0] [--duration 5m]", Message: "command.network.tcp", Example: "haco network tcp --target example.com --port 443"},
	{Path: "network udp", Syntax: "--target <name> [--port <port>] [--kind external|host|environment] [--listen 127.0.0.1:0] [--duration 5m]", Message: "command.network.udp", Example: "haco network udp --target <host> --port 53"},
	{Path: "network list", Syntax: "", Message: "command.network.list", Example: "haco network list"},
	{Path: "network revoke", Syntax: "<connection-id>", Message: "command.network.revoke", Example: "haco network revoke <connection-id>"},
	{Path: "network host", Syntax: "<command>", Message: "command.network.host", Example: "haco network host list"},
	{Path: "network host list", Syntax: "", Message: "command.network.host.list", Example: "haco network host list"},
	{Path: "network host add", Syntax: "--address <IP> --port <port> [--protocol tcp|udp] <name>", Message: "command.network.host.add", Example: "haco network host add --address 192.0.2.1 --port 8080 service"},
	{Path: "network host remove", Syntax: "<name>", Message: "command.network.host.remove", Example: "haco network host remove service"},
	{Path: "network rule", Syntax: "--env <name> --target <name> --decision allow|ask|deny [--port <port>] [--kind external|host|environment] [--protocol tcp|udp] [--duration 5m] [--ttl 1h] [--scope instance|environment|global]", Message: "command.network.rule", Example: "haco network rule --env dev --target example.com --port 443 --decision ask"},
	{Path: "aws", Syntax: "<command>", Message: "command.aws", Example: "haco aws s3 --help"},
	{Path: "aws s3", Syntax: "<command>", Message: "command.aws.s3", Example: "haco aws s3 ls s3://bucket/prefix"},
	{Path: "aws s3 ls", Syntax: "[--env <name>] [--profile <name>] [--region <region>] <s3://bucket/prefix>", Message: "command.aws.s3.ls", Example: "haco aws s3 ls s3://bucket/prefix"},
	{Path: "aws s3 cp", Syntax: "[--env <name>] [--profile <name>] [--region <region>] <s3://bucket/key> <file>", Message: "command.aws.s3.cp", Example: "haco aws s3 cp s3://bucket/key download"},
	{Path: "ssh", Syntax: "<command>", Message: "command.ssh", Example: "haco ssh setup dev"},
	{Path: "ssh setup", Syntax: "[environment]", Message: "command.ssh.setup", Example: "haco ssh setup dev"},
	{Path: "open", Syntax: "[--client vscode|ssh|none] [--repo <id[,id...]>] [--name <name>] [--base <base>] [--oci auto|none|oci:<store>] [--port <port>] [--close] [--no-browser] [environment-or-directory]", Message: "command.open", Example: "haco open --repo source ."},
}

func commandHelp(out io.Writer, path string, language cliui.Language) bool {
	return cliui.WriteCommandHelp(out, "haco", path, helpPages, language)
}

// Only literal command paths followed by help are intercepted. In particular,
// `haco run -- tool --help` remains an Execution, not Hacocoon help.
func requestedCommandHelp(args []string, out io.Writer) bool {
	if len(args) < 2 || (args[len(args)-1] != "--help" && args[len(args)-1] != "-h") {
		return false
	}
	return commandHelp(out, strings.Join(args[:len(args)-1], " "), cliLanguage())
}
