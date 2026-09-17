# Daily development workflow

[日本語](daily-workflow.ja.md) | English

Status: **implemented CLI workflow**. Provider/desktop acceptance is recorded
separately in [acceptance evidence](../status/acceptance-evidence.md#development-branch-integration).

## Add repositories, then open

After [installation](../guides/installation.md), run in the trusted management terminal:

```bash
haco repo add api https://github.com/OWNER/API.git
haco repo add web https://github.com/OWNER/WEB.git
haco open
```

Replace the URLs with your repositories. Normal open prepares independent files,
uses the default Base and configured storage, creates/reuses the Environment,
then opens VS Code with Remote-SSH. Use `haco open --client ssh` for a shell.
Repository credentials stay in trusted Host storage. Edit in `/workspace/api`
and `/workspace/web`; one repository uses `/workspace`.

Follow progress on stderr. If a permission request is pending, review it with
`haco approve` in another trusted terminal; waiting work continues afterward.
Default deny creates no prompt. Review required package/Git scopes with
`haco config --edit` and retry `haco open`. Neither registration nor open grants
push or unrestricted package/network access. See [first-use permissions](../guides/getting-started.md#review-required-permissions).

## Return to work

Run `haco open` again from the same management user's home. It reuses the same
project data and resumes a stopped Environment. Closing the editor or leaving
the shell does not stop it. For a deliberate stop, find the name with
`haco env list`, then run `haco env stop <environment>`; the next `haco open`
resumes it without a separate start command. Stop preserves installed tools,
rootfs, project edits and configured OCI data. Delete has different effects.

The initial default set contains one to eight repositories. Later registration
changes never overwrite existing work; use an explicit fork for a changed set.
See [default-session semantics](../design/default-development-session.md).

## Advanced selection and configuration

Existing explicit commands remain available. Open an Environment by name,
choose one with `haco open --select`, or use an owner-pinned directory reference
with `haco open .`. A new directory requires `--repo`; its contents are never
implicitly imported. [Workspace preparation and forks](../design/workspace-workflow.md)
and the [CLI reference](cli.md) cover explicit Workspace/Env/Base, storage,
network and configuration workflows. `--base` and `--oci` are also available on
the default open, subject to existing compatibility checks.

`haco ssh setup <environment>` prepares access without launching a client.
`haco open --client none --json` prepares/resumes the default work and returns
its identity without desktop preparation. Editor process launch is not proof
that the connection or build/test succeeded. See [Windows SSH](windows-environment-ssh.md).

## Targets, scripts and cancellation

Explicit lifecycle commands require a name. `open --select` and `ssh setup` can select
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
closed terminal. Temporary execution separately requests bounded cleanup and
reports whether cleanup is confirmed.

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

## Delete deliberately

In **trusted haco-host**, `haco env delete <environment>` prints the explicit target
and data effects before calling the existing canonical deletion API. It removes
the Env runtime/rootfs/connections and retains Workspace, OCI Stores and independent
snapshots. The explicit Env name remains the authorization intent; no new prompt
or incompatible command rename was added. If deletion fails, absence is not
confirmed and ownership/leases must remain governed by the lifecycle API.

Inspect retained data with `haco workspace list`, `haco plugin oci store list`
and `haco snapshot list`. The separate Workspace/Store delete commands display
what will disappear and require confirmation (or `--yes`). Workspace deletion
can remove uncommitted/untracked/unpushed work. Keep independent snapshots or
exports when that data is needed. This workflow does not redesign retention units.
