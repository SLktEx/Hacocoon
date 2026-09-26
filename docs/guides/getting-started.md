# First development Environment

[日本語](getting-started.ja.md) | English

This tutorial uses the current product `haco`, a managed GitHub repository and one
Environment. The local runtime is implemented; platform and feature acceptance
limits are listed in [current status](../IMPLEMENTATION_STATUS.md).

## Install and enter the Host

Use Ubuntu 26.04+ with systemd and Incus, or Windows with current WSL 2.
Choose the architecture-matching installer package from the project's
[releases](https://github.com/SLktEx/Hacocoon/releases).
Development snapshots are candidate builds, not published releases; for a
source build see [contributing](../../CONTRIBUTING.md).
Do not assume an older package contains everything documented on main.

On Windows, extract the complete ZIP, open PowerShell in that directory, and run:

```powershell
.\install-windows.bat
wsl -d Hacocoon
```

Installation prepares the dedicated distribution and controller. Ordinary
interactive WSL entry opens trusted `haco-host`. There is no native Windows
`haco.exe`. Explicit WSL commands stay on the Physical Host; they do not use
the interactive login route.

On native Ubuntu, extract the complete Ubuntu installer and run
`./install-ubuntu.sh`. Run product commands as the installed controller-group
user on that Physical Host. Source Git authentication belongs inside the trusted
Host. A product interactive trusted-Host shell entry is not currently available on native Ubuntu.
Native installation does not change the user's login shell.

[Installation and recovery](installation.md) covers interrupted registration,
current-package reruns, platform options and explicit root recovery.

In the trusted management terminal:

```bash
haco version --json
haco doctor
```

Record the build identity when reporting a problem. Doctor must complete all
checks successfully. A pending, failed or unavailable result is not readiness;
follow its diagnosis before creating work.

## Add repositories and open Haco

Run these commands in the **trusted management terminal** after installation.
Replace OWNER/API and OWNER/WEB with repositories you may use. One repository is
also sufficient. Private authentication stays in trusted `haco-host`: use
`gh auth login --hostname github.com --git-protocol https` there when needed.
Installation supplies Git and GitHub CLI as standard Host tools.

```bash
haco repo add api https://github.com/OWNER/API.git
haco repo add web https://github.com/OWNER/WEB.git
haco open
```

Install VS Code and Remote-SSH on the desktop for the default editor entry.
For a shell, use `haco open --client ssh` instead. Open prepares the independent
project files, default starting image, configured storage and Environment, then
prepares SSH access and launches the client. Follow the progress on the terminal.
No manual Workspace/Env creation or Base build is required.

With two repositories, edit `/workspace/api` and `/workspace/web`. A single
repository uses `/workspace`. Their independent Git metadata uses `haco://<id>`;
the managed broker is connected automatically. Host credentials are not copied.

## Review required permissions

Open does not grant network or Git permissions. When approval is pending, use
`haco approve` in a second **trusted management terminal** to review the exact
request. Waiting work continues after an authorized answer. If Policy denies the
operation, no approval prompt appears: use `haco config --edit` to review the
required scope, then retry `haco open`. Existing work is retained.

The initial SSH preparation may need to download `openssh-server`. Permit only
its actual package hostnames/protocols/ports for the Environment shown by
`haco env list`, following the [egress Policy example](../design/egress-authorization.md#policy-example).
Do not add an unrestricted wildcard. A Base with sshd already installed needs no
such download. A failed connection alone does not establish the cause; use
`haco doctor` and the displayed stage.

For fetch/pull and reviewed push, follow [scoped Git permissions](git-workflow.md#configure-git-policy).
Name resolution and connections remain separately controlled. Editing or building
locally does not grant remote write permission.

## Develop and return later

Inside the **Environment**, enter the appropriate repository:

```bash
cd /workspace/api
git status
# Edit files and run this repository's build/test commands.
```

Install project-specific tools through authorized package access or a
[setup recipe](../design/project-setup.md). If Git is absent from the selected
Base, install it there through that same permission path. Configure your Git
commit name/email in the Environment. Ordinary `git fetch`, `git pull --ff-only`
and `git push` use the broker; push requires the exact reviewed request, not
Host credentials. See [Git workflow](git-workflow.md).

Exit the shell or close the editor when finished. This does not stop or delete
the Environment. Next time, return to the trusted management terminal and run:

```bash
haco open
```

It reuses healthy work and resumes a stopped Environment. Unsaved editor buffers
still need saving; opening does not create backups or push commits.

## Explicit control and recovery

Normal use needs only registration and open. For deliberate lifecycle or custom
configuration, all [advanced CLI operations](../reference/cli.md) remain available:

```bash
haco env list
haco env stop <environment>
haco open
```

Use the actual name from the list. Stop retains installed packages, project edits
and configured persistent storage. Delete is different: `haco env delete <environment>`
discards the root filesystem but retains project files and OCI data. Read
[data lifetime](data-lifetime.md) before deletion. Explicit directory opens,
Base choices and independent forks are explained in [Workspace workflow](../design/workspace-workflow.md).

The default session supports one to eight repositories. Register the intended
set before the first open. Changing registrations later does not overwrite the
existing collection: use an explicit fork to preserve work while changing
membership. If ownership is incomplete, inspect the reported state; never edit
catalog files or guess provider cleanup targets. See the
[default-session contract and limits](../design/default-development-session.md).

Repository tests and real-host acceptance are separate. Fresh installed Incus,
Windows/WSL and desktop acceptance of this combined path remain pending.
