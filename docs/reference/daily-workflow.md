# Daily development workflow

[日本語](daily-workflow.ja.md) | English

Status: **implemented CLI workflow**. Provider/desktop acceptance is recorded
separately in [acceptance evidence](../status/acceptance-evidence.md#development-branch-integration).

## Add repositories, then open

After [installation](../guides/installation.md), run in the trusted management terminal:

```bash
haco repo add https://github.com/OWNER/API.git
haco open
```

Replace the URLs with your repositories. Normal open prepares independent files,
uses the default Image and configured storage, creates/reuses the Environment,
then opens VS Code with Remote-SSH. Use `haco open --client ssh` for a shell.
Repository credentials stay in trusted Host storage. One repository uses `/workspace`; collections use `/workspace/<repository-id>`.

Follow progress on stderr. If a permission request is pending, review it with
`haco approve` in another trusted terminal; waiting work continues afterward.
Default deny creates no prompt. Review required package/Git scopes with
`haco config --edit` and retry `haco open`. Neither registration nor open grants
push or unrestricted package/network access. See [first-use permissions](../guides/getting-started.md#review-required-permissions).

## Return to work

Run `haco open` again. It selects the last opened Environment, reuses its
project data and resumes it if stopped. Closing the editor or leaving
the shell does not stop it. For a deliberate stop, find the name with
`haco env list`, then run `haco env stop <environment>`; the next `haco open`
resumes it without a separate start command. Stop preserves installed tools,
rootfs, project edits and configured OCI data. Delete has different effects.

No registered repositories is valid and produces an empty Workspace. Later
registration changes affect future Environments only. Use `haco open --new`
for the current repository set, or `--snapshot SNAPSHOT` for saved data.
See [creation semantics](../design/environment-creation.md).

Inside the **Env**, edit under `/workspace` and run that repository's build/test
commands. For example, in a Go repository: `go build ./...` then `go test ./...`.
Network/package/Git operations still require the applicable Policy/approval.
For independent work, create another Environment with `haco open --new [IMAGE]`.

Existing explicit commands remain available. Open an Environment by name,
or use an owner-pinned directory reference
with `haco open .`. A new directory requires `--repo`; its contents are never
implicitly imported. [Workspace preparation and forks](../design/workspace-workflow.md)
and the [CLI reference](cli.md) cover explicit Workspace/Env/Base, storage,
network and configuration workflows. `--base` and `--oci` remain options for explicit directory workflows.
Use `haco image default [IMAGE]` for the default of new Environments.

`haco ssh setup <environment>` prepares access without launching a client.
`haco open --client none --json` prepares/resumes the default work and returns
its identity without desktop preparation. Editor process launch is not proof
that the connection or build/test succeeded. See [Windows SSH](windows-environment-ssh.md).

## Targets, scripts and cancellation

Explicit lifecycle commands require a name. `ssh setup` can select
from a numbered terminal list; a blank answer cancels before connection changes.
A single existing Env can be selected automatically. For scripts, always specify
the name; noninteractive ambiguous selection never waits for input. Put options
before positional targets. Command-specific `--help` shows the supported form.

Results remain on stdout; progress and diagnostics use stderr. Use
`haco env list --json` and `haco env status --json <environment>` for scripts.
Env creation retains its existing JSON result. Deletion of retained data needs
terminal confirmation or explicit `--yes`; a pipe/FIFO is never a confirmation
prompt. Ctrl+C may stop observation before a controller mutation finishes.
Inspect status before retrying; do not infer cancellation or cleanup from a
closed terminal. Failed creation cleans up newly owned resources; existing work is never rebuilt automatically.

## Failure and the next action

| What you see | What to do |
|---|---|
| setup running / succeeded / failed | These are observed stages, not percentages. Inspect the fixed stage/reason and request ID. |
| busy | Another operation owns the target; inspect/wait for that operation. It is not approval waiting. |
| pending Capability approval | In another trusted terminal use `haco approve` to list/review; observation never approves. |
| create/start/stop/delete/SSH failure | Resource state is unknown until `haco env status <environment>` and `haco doctor <environment>` inspect it. Do not assume cleanup succeeded. |
| editor launch failure after SSH preparation | Env/connection remain; use `haco open --client ssh <environment>` or fix the desktop editor. |
| setup customization failure | Substrate stages may have succeeded; the saved script and its side effects remain. Inspect/correct it before an explicit replay. `--clear-script` removes the saved recipe, not its side effects. |

For setup, run `haco doctor`. An administrator on the **WSL/Linux Physical Host**
can collect the controller journal without exposing it to Env workloads:

```bash
journalctl -u haco-controller.service --since '30 minutes ago' --no-pager
```

Locate the setup `request_id`. Raw backend output and secrets are not diagnostic
fields. A lost stream has no confirmed final result; controller setup may still
be running. See [setup diagnostics](../design/trusted-host.md#setup-progress-and-failure-diagnostics).

## Delete only what you intend

`haco env delete ENV` (or `haco rm ENV`) removes the Environment rootfs,
automatically owned Workspace and OCI data, without a confirmation prompt.
A running Environment requires `-f`, which stops it first. Explicit Volumes,
independent Workspaces and Snapshots survive. Inspect them with `haco volume ls`,
`haco workspace list` and `haco snapshot ls`. Independent data deletion retains
its command-specific confirmation rules. Uncommitted edits, untracked files and
unpushed commits in automatically owned work are deleted with the Environment;
retain needed data in a Snapshot, Volume or export first. Failed cleanup retains
ownership until absence is confirmed. See [data lifetime](../guides/data-lifetime.md).
