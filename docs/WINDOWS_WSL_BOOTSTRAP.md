# Windows / WSL installation

Hacocoon uses a **dedicated WSL 2 instance** on Windows instead of reusing a normal Ubuntu/Debian development distribution.

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

Windows and WSL lifecycle remain outside Hacocoon Core. Installation is handled by a host-side helper.

## Recommended installer

GitHub Releases publishes `install-windows.ps1` as the standalone Windows installer. It does **not** require a Hacocoon repository checkout.

Download that release asset to Windows and run it from an elevated PowerShell:

```powershell
.\install-windows.ps1
```

The default dedicated instance is:

```text
Hacocoon
```

and the default base distribution is:

```text
Ubuntu-26.04
```

The installer is resumable. On a fresh machine it may first create the WSL instance and stop because Windows needs a reboot or the new distribution needs first-launch Linux user creation. In that case:

```powershell
wsl -d Hacocoon
```

complete the Linux user setup, exit, and run `install-windows.ps1` again.

## What the standalone installer does

The installer:

1. validates the requested instance/base/version names;
2. reuses only a named `Hacocoon` instance if it already exists;
3. otherwise creates a new named WSL instance with `wsl --install <distro> --name <name> --no-launch`;
4. downloads `checksums.txt`, `bootstrap-wsl.sh`, and `install.sh` from the selected Hacocoon release;
5. verifies the downloaded Linux bootstrap scripts with SHA-256 values from the release checksum file;
6. runs the Linux bootstrap inside the dedicated WSL instance;
7. installs Incus plus `haco` and `haco-vscode`.

The release itself also publishes the bootstrap scripts as separate assets so the Windows installer does not need a repository checkout.

For a private repository, release download requires an authenticated `gh` CLI or `GH_TOKEN` / `GITHUB_TOKEN`. Public releases can be downloaded directly.

## Dedicated-instance rule

The installer never selects the user's default WSL distribution and never falls back to the first installed distribution.

If `Hacocoon` does not exist, the important platform operation is equivalent to:

```powershell
wsl --install Ubuntu-26.04 --name Hacocoon --no-launch
```

The installer does **not** run `wsl --set-default-version` and does not change the user's default WSL distribution. Existing Ubuntu, Debian, Arch, or other WSL instances remain user-owned state.

It never automatically:

- unregisters, resets, or deletes another WSL instance;
- converts WSL 1 to WSL 2;
- changes the default WSL distribution;
- changes global WSL defaults for future distributions;
- replaces another distribution's Linux user;
- rewrites arbitrary `/etc/wsl.conf` or Windows `.wslconfig` settings.

If the requested base distribution is unavailable, inspect the local WSL catalog with:

```powershell
wsl --list --online
```

and pass a valid name:

```powershell
.\install-windows.ps1 -BaseDistro Ubuntu
```

A different dedicated instance name may be used without reusing a general-purpose distribution:

```powershell
.\install-windows.ps1 -InstanceName Hacocoon-Dev
```

## Hacocoon version

The installer defaults to the latest Hacocoon release. A version can be pinned explicitly:

```powershell
.\install-windows.ps1 -HacocoonVersion v0.8.0
```

The Linux release installer still verifies the selected binary archive against `checksums.txt` before installing `haco` and `haco-vscode`.

## Incus authority is explicit

Installing Incus and granting control of the Incus daemon are separate operations.

The installer does **not** silently add the Linux user to `incus-admin`. Local Incus administrator access is effectively root-equivalent because it can attach host paths/devices and alter instance security.

When the workstation owner explicitly accepts that authority:

```powershell
.\install-windows.ps1 -GrantIncusAdmin
```

After group membership changes, restart the WSL shell or use `newgrp incus-admin` before relying on the new membership.

If Incus is already managed separately:

```powershell
.\install-windows.ps1 -SkipIncus
```

## Repository bootstrap for development

A source checkout still contains:

```powershell
.\scripts\bootstrap-windows.ps1
```

That path is useful while developing Hacocoon itself. It uses the checkout's local `bootstrap-wsl.sh` and `install.sh` instead of downloading release copies.

The normal user-facing path is the standalone `install-windows.ps1` release asset.

## Systemd and Incus initialization

The Linux bootstrap attempts to start the packaged Incus service when systemd is active. It applies `incus admin init --minimal` only when the daemon is reachable and no storage pool exists.

It does not rewrite `/etc/wsl.conf` or silently modify Windows `.wslconfig`. If host virtualization/systemd requirements are not satisfied, fix that platform configuration explicitly and rerun the installer.

## Workspace placement

Keep active local Hacocoon workspaces in the **dedicated Hacocoon WSL Linux filesystem** rather than another WSL distribution or `/mnt/c` by default.

```powershell
wsl -d Hacocoon
```

Then inside the dedicated instance:

```bash
mkdir -p ~/src
cd ~/src
git clone <repository>
cd <repository>
haco-vscode open .
```

This keeps Linux ownership, filesystem semantics, Incus bind-mount behavior, and Hacocoon tooling inside one explicit host boundary.

## VS Code

VS Code remains a Windows desktop client. Hacocoon does not add another AI UI.

```text
install-windows.ps1
  -> dedicated Hacocoon WSL 2
  -> Incus + Hacocoon
  -> workspace in dedicated WSL filesystem
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> /workspace inside the Hacocoon Environment
  -> existing VS Code AI / terminal / Git UI
```

## Acceptance boundary

CI validates PowerShell/shell syntax, release checksum inclusion, release packaging, and repository integration. It does not prove Windows feature enablement, reboot behavior, named distribution installation, first-run user setup, real WSL 2 behavior, real Incus startup, or desktop VS Code Remote-SSH.

Those remain real-host acceptance tests and must not be reported as passed until they run on an appropriate Windows + dedicated Hacocoon WSL 2 + Incus host.
