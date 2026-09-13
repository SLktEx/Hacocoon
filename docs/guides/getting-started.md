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
Host; the temporary explicit Host-shell entry is described in
[CLI migration](../reference/cli-migration.md#host-entry).
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

## Create independent project data

The following commands run in **trusted haco-host**, not in an Environment.
`haco setup` guarantees `git` and GitHub CLI (`gh`) as standard Host tools, so
no manual package installation is required. Authenticate with GitHub CLI only
when the selected repository needs credentials or you intend to push:

```bash
# For a private repository or a repository you can push to:
gh auth login --hostname github.com --git-protocol https
```

Authentication, dotfiles and additional personal or organization-specific tools
are not baked into the standard Host tool set.

The public example is sufficient for reading and local edits. For your own work,
replace the URL and branch with a repository and an **existing** branch you may use.
Keep the names consistent in the permission rules below.

```bash
haco repo clone --branch main sample https://github.com/SLktEx/Hacocoon.git
haco workspace create --repo sample sample-work
haco env create --workspace managed:sample-work sample-dev
haco env status sample-dev
```

The source checkout stays in trusted Host storage. The Workspace gets independent
files and `.git`, mounted at `/workspace` in the Environment. Its remote uses
`haco://sample`; creating the Environment automatically wires the managed Git
broker. This local wiring does not contact the upstream remote; network activity
starts when the Environment later runs Git operations such as fetch or push.

Creation uses the default Base and, when configured, independently copies/reuses
the Workspace's OCI Store. For a project without container tooling,
`haco env create --no-oci --workspace managed:sample-work sample-dev` is the
explicit alternative. Do not run both creates. A configured copy failure is
reported, not converted into an empty Store. See [OCI Store limits](../design/persistent-oci-store.md).

## Permit only the needed network operations

Run `haco config --edit` in the **trusted management terminal**. Preserve the
revision and existing Policy rules. [Configuration](../reference/configuration.md)
explains how edits and saved approvals interact.

Hacocoon official Bases contain OpenSSH server when they are published through
the Base Builder. A fresh Environment created from an official Base therefore
does **not** need Ubuntu package-mirror permission merely for `haco ssh setup` or
`haco open --client ssh`, and SSH setup does not run `apt-get` in the Environment.
A custom Base must itself provide a compatible `sshd` and systemd SSH unit. If it
does not, SSH setup fails explicitly; add the required package while building the
Base rather than widening the Environment's network Policy.

For ordinary fetch/pull and reviewed push, add the scoped rules from
[managed Git](git-workflow.md#configure-git-policy), replacing the URL and branch
with your registration. A separate `network.resolve/lookup` rule is needed for
applications that resolve names themselves; [name resolution](../design/name-resolution.md)
does not grant a connection. Host-brokered Git does not need guest GitHub credentials.

## Connect and develop

From the **trusted management terminal**:

```bash
haco open --client ssh sample-dev
```

This prepares desktop-owned SSH keys, pins the provider-supplied host key, and
opens a shell in `/workspace`. On Windows it uses Windows OpenSSH through WSL
interop; keep the ordinary Host terminal open. For VS Code, install VS Code and
Remote-SSH on the desktop, then use `haco open sample-dev`.
[SSH details](../reference/windows-environment-ssh.md) cover manual clients and failures.

Inside the **Environment SSH session**:

```bash
cd /workspace
apt-get update
apt-get install -y git
git status
git fetch origin
git pull --ff-only
# Edit files and run this project's build/test commands.
git config user.name 'Your Name'
git config user.email 'your-address@example.com'
git add <files>
git commit -m 'Describe the change'
```

Replace `<files>` with the intended files. The managed SSH configuration supplies
the credential-free proxy settings; permission still comes from Policy.
Project-specific tools can be installed in this Environment or supplied by a Base.
[Setup recipes](../design/project-setup.md) make dependency installation repeatable.

Push only to a repository/branch you can write. In the Environment, run `git push`.
While it waits, open a second **trusted Host terminal**, run `haco git pending`,
review the exact remote/ref and old/new commits, then use
`haco git approve <id>` or `haco git deny <id>`.
The public Hacocoon example does not grant you upstream push permission.
See [Git approvals and ambiguous results](git-workflow.md).

## Stop and return later

Exit the SSH shell, then run in the **Host**:

```bash
haco env stop sample-dev
haco env status sample-dev
```

Expected result: stopped, with the Workspace retained. Stop retains the lease,
root filesystem, Git edits and optional Store. Guest `/tmp` may be cleared by
the guest OS at boot. Exiting a shell alone does not stop the Environment.
For explicit access revocation, use `haco env disconnect sample-dev <connection-id>`.

Later, enter the Host again and run:

```bash
haco open --client ssh sample-dev
```

Opening resumes a stopped Environment through ownership and network checks.
`haco env start sample-dev` is also available. After upgrading old instances,
apply normal stop/start before a Host reboot as described in
[the lifecycle contract](../design/workspace-abstraction-and-lease.md#explicit-start-after-a-physical-host-boot).

## Delete only when intended

`haco env delete sample-dev` destroys Environment-only files and packages but
retains the Workspace and OCI Store. It does not push commits or make a backup.
Use [data lifetime and cleanup](data-lifetime.md) for recreation, snapshots and
separate deletion of retained data.

If creation, copying, start or cleanup reports uncertain ownership, stop and
inspect `haco env list`, `haco workspace list` and the reported resource IDs.
Do not edit the catalog, delete guessed Incus paths or disable isolation to continue.
General interrupted-operation recovery remains incomplete.