# Product CLI reference

[日本語](cli.ja.md) | English

This page describes `cmd/haco-product`, installed as `haco`.
Run management commands in trusted `haco-host` or on its Physical Host.
Paths passed as input files are client-local; `env create --workspace` instead
resolves a path on the controller (or a `managed:<id>` identity).
Options normally precede positional arguments. `haco help` lists command groups;
help and version require no controller.

| Purpose | Syntax and defaults | Details |
|---|---|---|
| Workspace path | `haco workspace prepare --path <dir> --repo <id[,id...]> [--name <name>] [--oci auto\|none\|oci:ID]`; `haco workspace fork --path <new-dir> [--name <name>] <source-dir>`; `haco open [--repo <ids>] [--client vscode\|ssh\|none] <dir>` | [Owner-pinned entry and independent data forks](../design/workspace-workflow.md) |
| TCP/UDP | `haco network tcp\|udp`, `host add\|remove`, `rule`, `list`, `revoke`; `haco env forward --protocol tcp\|udp --target-port <port> <env>` | [Exact options, guest listeners and management authority](../design/network-connections.md) |
| Build identity | `haco version [--json]`, `haco --version` | [Build identity](build-release-identity.md) |
| Host/project setup | `haco setup [--script <path> \| --clear-script] [environment]` | [Host](../design/trusted-host.md), [project](../design/project-setup.md); omitted target means trusted Host; Host-only `haco setup --reapply-script` and `haco setup --script-result` reapply or inspect user customization |
| Diagnostics | `haco doctor [--json] [environment]` | Host by default; failed/skipped checks return nonzero |
| Policy | `haco config`, `--edit` or `--file <json>` | [Configuration](configuration.md) |
| Experimental VS Code | `haco experimental edit vscode [--file <yaml> \| --json [ - ]]` | [Subtree editing and Env application](experimental-vscode.md) |
| Approval | `haco approve [--json] [request-id]`; `haco approve --list` | [Review](../design/pending-approval-review.md); interactive selection/saved choices |
| Source | `haco repo clone --branch <branch> <id> <URL>`; `list [--json]`; `delete [--yes] <id>` | [Git](../guides/git-workflow.md); existing branch required |
| Workspace | `haco workspace create --repo <id[,id...]> <workspace>`; `list [--json]`; `delete [--yes] <id>` | Independent Git/data copies |
| Create | `haco env create --workspace <path-or-managed:id> [--base <base>] [--resource oci:<store> \| --no-oci] <name>` | Default Base; optional configured OCI initialization |
| Inspect | `haco env list [--json]`; `haco env status [--json] <name>` | Text by default |
| Lifecycle | `haco env start <name>`, `stop <name>`, `delete <name>` | [Data lifetime](../guides/data-lifetime.md) |
| Desktop | `haco ssh setup [environment]`; `haco ssh cleanup`; `haco open [--client vscode\|ssh] [environment]` | VS Code default; stopped Env resumes; interactive choice if ambiguous |
| Manual SSH | `haco env ssh --key <public-key-file> <name>`; `ssh-config <name>`; `disconnect <name> <connection-id>` | ProxyCommand uses a durable target; `haco stream <target>` exposes raw stdio; [SSH](windows-environment-ssh.md) |
| Preview | `haco open --port <port> [--close \| --no-browser] [environment]` | [HTTP preview](../design/development-preview.md); Env loopback port |
| Temporary command | `haco run [-i \| -it] [--workspace <workspace>] [--base <base>] [--no-oci] [--read-only] [--json] -- <command...>` | [Temporary execution](../design/temporary-execution.md); `--rm` defaults true; `-i` streams input, `-it` uses a terminal; JSON is captured-output only |
| Base | `haco base list`; `list --all [--json]`; `inspect <base>`; `build <definition.json>`; `delete [--yes] <name-or-fingerprint>` | [Base](../design/base-images-and-custom-environments.md); ordinary list/inspect return JSON |
| Git broker | `haco git connect <env>`; `pending`; `approve [--save env\|all\|ask-env\|ask-all] <id>`; `deny [--save ...] <id>` | [Git approvals](../guides/git-workflow.md) |
| OCI Store | `haco plugin oci store create <id> [--from <id>]`; `inspect <id>`; `list [--json]`; `delete [--yes] <id>` | [Store](../design/persistent-oci-store.md); `--from` also accepted before target |
| OCI images | `haco plugin oci image list [--unused] [--runtime nerdctl\|docker] [--json] [--host] [<env-or-store-id>]` | [Image reference](../design/oci-image-deletion.md); nerdctl default; `--host` replaces target |
| Image removal | `haco plugin oci image delete [--unused] [--runtime nerdctl\|docker] [--yes] [--host] [<env-or-store-id>] [<image-id-or-tag>]` | `--unused` replaces image selector; reviewed candidates may include tagged images |
| Snapshot | `haco snapshot create [--json] <env>`; `list [--json] [env]`; `inspect [--json] [--details] <id>`; `restore [--json] <id> [new-env]`; `delete <id>` | [Snapshots](../design/environment-snapshots.md) |
| Copy | `haco env copy [--json] <stopped-env> [new-env]` | Default `<source>-copy`; [copy](../design/environment-copy.md) |
| Transfer | `haco env export [--json] <stopped-env> [file.haco]`; `import [--json] <file.haco> [new-env]` | Linux; defaults `<env>.haco` / `<source>-imported`; [transfer](../design/environment-transfer.md) |
| Disk allocation | `haco reclaim [--yes \| --status \| --review [--yes]]` | Managed Windows/WSL only; [reclamation](../design/storage-reclamation.md) |
| AWS | `haco aws s3 ls [--env <name>] [--profile <name>] [--region <region>] s3://bucket/prefix`; `haco aws s3 cp [same options] s3://bucket/key <file>` | [AWS](../design/aws-operations.md); profile default `default`; guest forbids `--env` |

`haco env switch-base` is explicitly disabled. Product `haco` has no top-level
`create/exec/shell/events/connections/forward`, no `plugin git` or `plugin oci seed/docker`,
and no CPU/memory/PID/root-size flags on `env create` or `run`.
Those retained legacy surfaces are covered by [migration](cli-migration.md).

Ordinary failure exits nonzero; usage usually exits 2. Temporary execution returns
the guest exit code after confirmed cleanup, 130 after client cancellation, and
failure for uncertain cleanup. A dispatch receipt (`reclaim`) is not completion.
Use JSON only on commands that explicitly provide it; there is no global `--json`.

## Hierarchical help

The development candidate uses shared list/detail help for Env, repo, Workspace,
Git, Base, snapshot, OCI, network, AWS, SSH and open. `haco env --help` lists one
command and purpose at a time; `haco env create --help` shows its own syntax and
example. Explicit `--help`/`-h` prints to stdout and exits zero before controller
or Incus access. Invalid arguments retain stderr and nonzero exits. Explanations
wrap with aligned indentation at 60 columns. Wrapping long command syntax and
copyable examples, and companion haco-host help, remain follow-up work.
See [language coverage](cli-language.md).

`haco git status [--json] [--request <request-id>] <environment>` shows the latest or selected recorded push. `haco git reconcile` accepts the same arguments and requests a fresh Policy-controlled read. Neither repeats a push. See [Git recovery](../guides/git-workflow.md).

## Cache commands

On the trusted Host, `haco cache settings` displays configured areas, `haco cache configure <file>` applies a JSON document to future Environments, `haco cache status <env>` shows origins/current generations and `haco cache collect <stopped-env> [area]` collects complete areas. Put `--json` before the target. Existing contents are not adopted; history/clear/completed-copy recovery and additional-data transfer are implemented candidates. See [configuration and ordinary use](../design/cache-generations.md#configure-and-collect).

## Build a Base with Packer

```sh
haco base build --name my-tools [--from haco/ubuntu-26.04] [--output] [--json] <directory>
```

The directory contains HCL2 and external scripts. Options precede it. See [Packer builds](../design/packer-base-builds.md) for dependencies, data selection, results and recovery.

### Cache history and clear

On the trusted Host, use `haco cache history [--json] <env> <area>` to inspect
retained collection attempts. `haco cache clear [--yes] [--json] <env> <area>`
reviews and resets that reuse source, then removes eligible reviewed source data.
Existing Environment copies, Workspace and OCI data remain. Shared scope affects
the whole group's future Environments. Partial cleanup returns nonzero and retains
its result in JSON; inspect history before retrying. See [cache generations](../design/cache-generations.md#inspect-and-clear-collected-data).

`haco cache recover [--json] <env> <area>` resumes positively completed collection through exact ownership and provider verification. It does not prompt for deletion or copy data again. A current source is reusable; stale completed candidates remain retained. Unknown completion remains an error. See [recovery](../design/cache-generations.md#recover-completed-collection).

## Local application tunnel

Run `haco env tunnel --target-port 8080 demo` from ordinary Host entry and connect an application to the address it prints. The default port is automatic, the maximum duration is one hour, and Ctrl+C closes all connections. Native Linux listens locally; WSL entry uses the installed Windows client. Direct PowerShell use: `& <installed-haco-tunnel.exe> --distribution <WSL-name> --target-port 8080 demo`. The installer prints its location. See [transport and prerequisites](../design/controller-client-transport.md#client-tcp-listeners).

Environment creation accepts `--dns host|backend|disabled` (default `host`); ordinary status displays the selected resolver. Snapshot/copy/transfer preserve the mode. See [name resolution](../design/name-resolution.md) for the Policy boundary and acceptance scope.

## Base archive input

`haco base import --name <base> [--json] <image.tar>` imports an uncompressed Incus container image through an isolated temporary Env and publishes an immutable Base. The source file remains.

See [input, limits and failure handling](../design/base-images-and-custom-environments.md#import-a-container-image-archive).

## Empty Env cache contents

Use `haco cache empty --preview <env> [<area>]` to inspect enrolled areas, stop the
Env, then run `haco cache empty [--yes] [--json] <env> [<area>]`. `--all` selects all
Envs. Workspace, OCI, common generations and saved copies remain. Keep an interrupted
Env stopped and explicitly retry after fresh review. See [scope and limits](../design/cache-generations.md#empty-an-environments-cache).
