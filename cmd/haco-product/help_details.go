package main

import "github.com/SLktEx/Hacocoon/internal/cliui"

// Presentation metadata is attached once. Parsing, default values, lifecycle and
// approval decisions remain in their existing command/service owners.
func init() {
	field := func(syntax, message string) cliui.HelpField { return cliui.HelpField{Syntax: syntax, Message: message} }
	json := field("--json", "flag.json")
	yes := field("--yes", "detail.yes")
	base := field("--base <base>", "detail.base")
	noOCI := field("--no-oci", "flag.no_oci")
	env := field("<name>", "detail.env")
	snapshot := field("<snapshot-id>", "detail.snapshot")
	connection := field("<connection-id>", "detail.connection")
	path := field("--path <directory>", "detail.path")
	name := field("--name <name>", "detail.work_name")
	repos := field("--repo <id[,id...]>", "detail.repos")
	oci := field("--oci auto|none|oci:<store>", "detail.oci")
	protocol := field("--protocol tcp|udp", "detail.protocol")
	port := field("--port <port>", "detail.target_port")
	kind := field("--kind external|host|environment", "detail.kind")
	target := field("--target <name>", "detail.target")
	duration := field("--duration <duration>", "detail.duration")
	store := field("<store>", "detail.store")
	baseName := field("<base>", "detail.base_name")
	set := func(paths []string, arguments, options []cliui.HelpField, notes ...string) {
		for i := range helpPages {
			for _, path := range paths {
				if helpPages[i].Path == path {
					helpPages[i].Arguments = arguments
					helpPages[i].Options = options
					helpPages[i].Notes = notes
				}
			}
		}
	}
	set([]string{"env list", "repo list", "workspace list", "plugin oci store list"}, nil, []cliui.HelpField{json})
	set([]string{"version"}, nil, []cliui.HelpField{json})
	set([]string{"git status", "git reconcile"}, []cliui.HelpField{env}, []cliui.HelpField{json, field("--request <request-id>", "git.recovery.request")}, "git.recovery.next")
	set([]string{"doctor"}, []cliui.HelpField{field("[environment]", "detail.doctor_env")}, []cliui.HelpField{json})
	set([]string{"setup"}, []cliui.HelpField{field("[environment]", "detail.setup_env")}, []cliui.HelpField{field("--script <path>", "detail.setup_script"), field("--clear-script", "detail.setup_clear")})
	set([]string{"config"}, nil, []cliui.HelpField{field("--edit", "detail.config_edit"), field("--file <file>", "detail.config_file")})
	set([]string{"approve"}, []cliui.HelpField{field("[request-id]", "detail.approve_request")}, []cliui.HelpField{field("--list", "approval.flag_list"), field("--json", "approval.flag_json")})
	set([]string{"reclaim"}, nil, []cliui.HelpField{field("--yes", "detail.reclaim_yes"), field("--status", "detail.reclaim_status"), field("--review", "detail.reclaim_review")}, "detail.reclaim_before")
	set([]string{"env create"}, []cliui.HelpField{field("<name>", "detail.env_new")}, []cliui.HelpField{field("--workspace <workspace>", "detail.workspace_required"), base, field("--resource oci:<store>", "flag.resource"), noOCI}, "detail.retention")
	set([]string{"env status"}, []cliui.HelpField{env}, []cliui.HelpField{json})
	set([]string{"env start", "env stop", "env delete", "env ssh-config", "git connect"}, []cliui.HelpField{env}, nil, "detail.retention")
	set([]string{"env ssh"}, []cliui.HelpField{env}, []cliui.HelpField{field("--key <public-key-file>", "flag.ssh_key"), field("--port <port>", "flag.ssh_port")})
	set([]string{"env disconnect"}, []cliui.HelpField{env, connection}, nil)
	set([]string{"env forward"}, []cliui.HelpField{env}, []cliui.HelpField{field("--target-port <port>", "detail.target_port"), protocol, field("--port <local-port>", "flag.ssh_port")})
	set([]string{"env copy"}, []cliui.HelpField{field("<stopped-env>", "detail.stopped"), field("[new-env]", "detail.copy_name")}, []cliui.HelpField{json})
	set([]string{"env export"}, []cliui.HelpField{field("<stopped-env>", "detail.stopped"), field("[file.haco]", "detail.archive_out")}, []cliui.HelpField{json})
	set([]string{"env import"}, []cliui.HelpField{field("<file.haco>", "detail.archive_in"), field("[new-env]", "detail.import_name")}, []cliui.HelpField{json})
	set([]string{"repo clone"}, []cliui.HelpField{field("<id>", "detail.repo_new"), field("<URL>", "detail.remote")}, []cliui.HelpField{field("--branch <branch>", "detail.branch")}, "detail.read_access")
	set([]string{"repo delete"}, []cliui.HelpField{field("<id>", "detail.repo")}, []cliui.HelpField{yes})
	set([]string{"workspace create"}, []cliui.HelpField{field("<workspace>", "detail.workspace_new")}, []cliui.HelpField{repos})
	set([]string{"workspace prepare"}, nil, []cliui.HelpField{path, repos, name, base, oci})
	set([]string{"workspace fork"}, []cliui.HelpField{field("<source>", "detail.workspace_source")}, []cliui.HelpField{path, name, field("--base <base>", "detail.fork_base")})
	set([]string{"workspace delete"}, []cliui.HelpField{field("<workspace>", "detail.workspace")}, []cliui.HelpField{yes})
	set([]string{"git approve", "git deny"}, []cliui.HelpField{field("<id>", "detail.request")}, []cliui.HelpField{field("--save env|all|ask-env|ask-all", "detail.saved")}, "detail.read_access")
	set([]string{"base list"}, nil, []cliui.HelpField{field("--all", "detail.base_all"), json})
	set([]string{"base inspect"}, []cliui.HelpField{baseName}, nil)
	set([]string{"base build"}, []cliui.HelpField{field("<definition.json>", "detail.definition")}, nil)
	set([]string{"base delete"}, []cliui.HelpField{field("<name-or-fingerprint>", "detail.base_delete")}, []cliui.HelpField{yes})
	set([]string{"snapshot create"}, []cliui.HelpField{env}, []cliui.HelpField{json})
	set([]string{"snapshot list"}, []cliui.HelpField{field("[env]", "detail.snapshot_env")}, []cliui.HelpField{json})
	set([]string{"snapshot restore"}, []cliui.HelpField{snapshot, field("[new-env]", "detail.restore_name")}, []cliui.HelpField{json})
	set([]string{"snapshot delete"}, []cliui.HelpField{snapshot}, nil)
	set([]string{"plugin oci store inspect"}, []cliui.HelpField{store}, nil)
	set([]string{"plugin oci store create"}, []cliui.HelpField{field("<store>", "detail.store_new")}, []cliui.HelpField{field("--from <store>", "detail.store_from")})
	set([]string{"plugin oci store delete"}, []cliui.HelpField{store}, []cliui.HelpField{yes})
	imageTarget := field("[<env-or-store-id>]", "detail.image_target")
	runtime := field("--runtime nerdctl|docker", "detail.runtime")
	hostImages := field("--host", "detail.host_images")
	set([]string{"plugin oci image list"}, []cliui.HelpField{imageTarget}, []cliui.HelpField{field("--unused", "detail.unused_list"), runtime, json, hostImages})
	set([]string{"plugin oci image delete"}, []cliui.HelpField{imageTarget, field("[<image-id-or-tag>]", "detail.image")}, []cliui.HelpField{field("--unused", "detail.unused_delete"), runtime, yes, hostImages})
	set([]string{"network tcp", "network udp"}, nil, []cliui.HelpField{target, field("--port <port>", "detail.network_port"), kind, field("--listen <address>", "detail.listen"), duration})
	set([]string{"network revoke"}, []cliui.HelpField{connection}, nil)
	set([]string{"network host add"}, []cliui.HelpField{field("<name>", "detail.service_new")}, []cliui.HelpField{field("--address <IP>", "detail.host_address"), port, protocol})
	set([]string{"network host remove"}, []cliui.HelpField{field("<name>", "detail.service")}, nil)
	set([]string{"network rule"}, nil, []cliui.HelpField{field("--env <name>", "detail.rule_env"), target, field("--port <port>", "detail.network_port"), field("--decision allow|ask|deny", "detail.decision"), kind, protocol, duration, field("--ttl <duration>", "detail.ttl"), field("--scope instance|environment|global", "detail.scope")})
	aws := []cliui.HelpField{field("--env <name>", "detail.aws_env"), field("--profile <name>", "detail.aws_profile"), field("--region <region>", "detail.aws_region")}
	set([]string{"aws s3 ls"}, []cliui.HelpField{field("<s3://bucket/prefix>", "detail.s3_prefix")}, aws)
	set([]string{"aws s3 cp"}, []cliui.HelpField{field("<s3://bucket/key>", "detail.s3_object"), field("<file>", "detail.download")}, aws)
	set([]string{"ssh setup"}, []cliui.HelpField{field("[environment]", "detail.env_optional")}, nil)
	set([]string{"open"}, []cliui.HelpField{field("[environment-or-directory]", "detail.open")}, []cliui.HelpField{field("--client vscode|ssh|none", "detail.client"), field("--repo <id[,id...]>", "detail.repos_optional"), name, base, oci, field("--port <port>", "detail.preview_port"), field("--close", "detail.close_preview"), field("--no-browser", "detail.no_browser")})
	set([]string{"run"}, []cliui.HelpField{field("-- <command...>", "detail.command")}, []cliui.HelpField{field("-i, --interactive", "run.flag_input"), field("-t, --tty, -it", "run.flag_tty"), field("--workspace <workspace>", "run.flag_workspace"), base, noOCI, field("--read-only", "run.flag_readonly"), json, field("--rm", "run.flag_rm")}, "run.help")
}
