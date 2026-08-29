# Windows / WSL bootstrap

Hacocoon runs its local runtime on Linux. On Windows, the supported local shape uses a **dedicated WSL 2 instance for Hacocoon** instead of reusing the user's normal Ubuntu/Debian environment:

```text
Windows desktop
  |
  +-- normal user WSL distributions      <- untouched
  |
  +-- WSL 2 instance: Hacocoon           <- dedicated Hacocoon host
        -> Incus
           -> Hacocoon Environment

Windows VS Code
  -> haco-vscode
  -> Remote-SSH
  -> Hacocoon Environment
```

The bootstrap scripts make host setup repeatable without moving Windows or WSL lifecycle into Hacocoon Core.

## Bootstrap entrypoint

From a PowerShell prompt in a Hacocoon checkout:

```powershell
.\scripts\bootstrap-windows.ps1
```

The default dedicated instance name is:

```text
Hacocoon
```

The default base distribution is `Ubuntu-26.04`. A different base may be selected explicitly:

```powershell
.\scripts\bootstrap-windows.ps1 -BaseDistro Ubuntu
```

The instance name may also be changed without reusing another general-purpose WSL instance:

```powershell
.\scripts\bootstrap-windows.ps1 -InstanceName Hacocoon-Dev
```

Modern WSL supports installing another instance of the same distribution under a custom name with `wsl --install <distro> --name <name>`. Hacocoon uses that model so existing WSL distributions remain separate.

## Dedicated-instance rule

The bootstrap does **not** select the default WSL distribution and does not fall back to the first installed distribution.

If `Hacocoon` already exists, the bootstrap reuses that named instance. Otherwise it creates it with the equivalent of:

```powershell
wsl --set-default-version 2
wsl --install Ubuntu-26.04 --name Hacocoon --no-launch
```

`-WebDownload` may be supplied when the local WSL installation needs the web-download path.

The script never unregisters, resets, deletes, converts, or mutates unrelated WSL distributions. It also does not change the user's default WSL distribution.

If the requested base distribution is not available through the local WSL catalog, inspect the supported names with:

```powershell
wsl --list --online
```

and pass a valid name through `-BaseDistro`.

## Fresh-machine flow

Installing WSL components or a new distribution may require a Windows reboot. A newly created distribution may also require first-launch Linux user creation.

For that reason a fresh-machine run may stop after creating the dedicated instance. If instructed, run:

```powershell
wsl -d Hacocoon
```

complete the Linux user setup, exit, and run the bootstrap again.

This is intentionally a resumable two-step flow rather than trying to automate Windows reboot or invent Linux credentials.

## What is installed inside the dedicated WSL instance

For an apt-based distribution, `scripts/bootstrap-wsl.sh` installs:

- CA certificates;
- `curl`;
- `tar`;
- `git`;
- Incus, unless `-SkipIncus` is used.

It then delegates Hacocoon installation to `scripts/install.sh`, which installs:

```text
haco
haco-vscode
```

A specific Hacocoon version may be requested:

```powershell
.\scripts\bootstrap-windows.ps1 -HacocoonVersion v0.8.0
```

Private-repository release downloads still require authentication accepted by `scripts/install.sh`.

## Incus authority is explicit

Installing Incus and granting control of the Incus daemon are different operations.

The bootstrap does **not** silently add the Linux user to `incus-admin`. Local Incus administrator access is effectively root-equivalent authority because it can attach host paths/devices and alter instance security.

When the workstation owner explicitly wants the dedicated Hacocoon WSL user to control Incus:

```powershell
.\scripts\bootstrap-windows.ps1 -GrantIncusAdmin
```

After group membership changes, restart the WSL shell or use `newgrp incus-admin` before relying on the new membership.

If Incus is managed separately:

```powershell
.\scripts\bootstrap-windows.ps1 -SkipIncus
```

## Systemd and Incus initialization

The bootstrap attempts to start the packaged Incus service when systemd is active. It applies `incus admin init --minimal` only when the daemon is reachable and no storage pool exists.

The bootstrap does not rewrite `/etc/wsl.conf` or silently alter global `~/.wslconfig` settings. If systemd or virtualization requirements are not satisfied, fix that host configuration explicitly and re-run the bootstrap.

## Workspace placement

Keep active Hacocoon workspaces in the **dedicated Hacocoon WSL Linux filesystem**, not in another WSL distribution and not under `/mnt/c` by default.

Example:

```powershell
wsl -d Hacocoon
```

then inside that instance:

```bash
mkdir -p ~/src
cd ~/src
git clone <repository>
cd <repository>
haco-vscode open .
```

This keeps Linux ownership, filesystem semantics, Incus bind-mount behavior, and Hacocoon tooling inside one explicit host boundary.

## VS Code

VS Code remains a Windows desktop client. The bootstrap does not create another AI UI or make VS Code part of Core.

```text
PowerShell bootstrap
  -> dedicated Hacocoon WSL 2 instance
  -> Incus + Hacocoon
  -> workspace in Hacocoon WSL filesystem
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> /workspace inside the Hacocoon Environment
  -> existing VS Code AI / terminal / Git UI
```

## Acceptance boundary

CI validates script syntax and repository integration. It does not prove Windows feature enablement, reboot behavior, distribution download, first-run user setup, real WSL 2 behavior, real Incus startup, or desktop VS Code Remote-SSH.

Those remain real-host acceptance tests and must not be reported as passed unless they were run on an appropriate Windows + dedicated Hacocoon WSL 2 + Incus host.
