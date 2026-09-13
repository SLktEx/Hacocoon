# Daily development workflow

[日本語](daily-workflow.ja.md) | English

Status: **implemented CLI workflow**. Provider/desktop acceptance is recorded
separately in [acceptance evidence](../status/acceptance-evidence.md#development-branch-integration).

## Prepare once

Install with the [Windows/WSL installer](../guides/installation.md). In a
Windows terminal, `wsl -d <your-installed-distribution>` opens trusted
`haco-host` when the managed login entry is installed. `haco-host` is trusted
management infrastructure; run untrusted tools in an Env. Product management
commands also work from the WSL/Linux Physical Host against the same controller.

In **trusted haco-host**, verify readiness and prepare a managed repository:

```bash
haco doctor
haco repo clone --branch main sample https://github.com/OWNER/REPO.git
haco workspace create --repo sample sample-work
haco env create --workspace managed:sample-work sample-dev
haco open sample-dev
```

Replace OWNER/REPO and the existing branch. Private Git authentication stays in
trusted haco-host; see [managed repositories](../guides/git-workflow.md).
Base selection defaults to the configured Base. An optional OCI Store copy is
automatic; `--no-oci` skips it. There is no required OCI runtime for Core.

For existing files on the **WSL/Linux Physical Host**, an alternative is
`haco env create --workspace /absolute/path/to/work sample-dev`. The path belongs
to that Physical Host, not Windows or the haco-host container. Writable files
are writable by Env workloads. `haco open .` can instead reopen an explicitly prepared, owner-pinned managed
Workspace reference; it does not copy or mount the directory contents. See
[Workspace preparation and forks](../design/workspace-workflow.md).

`haco open` prepares desktop-owned SSH keys/settings and launches the configured
editor. `haco open --client ssh sample-dev` opens a standard SSH shell instead.
Use `haco ssh setup sample-dev` to prepare SSH without launching a client.
Desktop keys stay on the desktop. Editor process launch does not prove that its
connection or build/test is ready. See [Windows SSH](windows-environment-ssh.md).

Before the first `open`, a Base without `sshd` needs package access to install
`openssh-server`. In **trusted haco-host**, `haco config` shows current Policy and
`haco approve --list` lists pending requests without deciding. No pending request
does not mean network access is allowed: default deny creates no approval prompt.
Use `haco config --edit` to review the existing snapshot and add only the required
Env/hostname/protocol/port rules; preserve other rules and default deny. Ubuntu's
default package sources use `archive.ubuntu.com` and `security.ubuntu.com`; inspect
your Base for mirrors. See the [Policy example](../design/egress-authorization.md#policy-example).
An SSH failure does not establish its cause. Inspect Env state/connections and
Policy before explicitly preparing SSH again. Successfully installed packages
remain in the Env rootfs across stop/start.

## Work, stop and return tomorrow

Inside the **Env**, edit under `/workspace` and run that repository's build/test
commands. For example, in a Go repository: `go build ./...` then `go test ./...`.
Network/package/Git operations still require the applicable Policy/approval.
`haco run --no-oci -- <command>` is a separate temporary-Env execution, not an
execution inside the named persistent Env.

Exit the Env shell or return to the **trusted haco-host** terminal:

```bash
haco env stop sample-dev
# Tomorrow, in trusted haco-host:
haco env list
haco env status sample-dev
haco env start sample-dev
haco open sample-dev
```

Stop keeps the Env rootfs, Workspace and OCI data. Do not substitute delete for
stop when you want to resume the same installed tools/rootfs tomorrow.

## Targets, scripts and cancellation

Mutation commands require an explicit name. `open` and `ssh setup` can select
from a numbered terminal list; a blank answer cancels before connection changes.
A single existing Env can be selected automatically. For scripts, always specify
the name; noninteractive ambiguous selection never waits for input. Put options
before positional targets. Command-specific `--help` shows the supported form.

Results remain on stdout; progress and diagnostics use stderr. Use
`haco env list --json` and `haco env status --json sample-dev` for scripts.
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
| create/start/stop/delete/SSH failure | Resource state is unknown until `haco env status sample-dev` and `haco doctor sample-dev` inspect it. Do not assume cleanup succeeded. |
| editor launch failure after SSH preparation | Env/connection remain; use `haco open --client ssh sample-dev` or fix the desktop editor. |
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

In **trusted haco-host**, `haco env delete sample-dev` prints the explicit target
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
