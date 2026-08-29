# Windows / WSL installation

Hacocoon uses a **dedicated WSL 2 instance with systemd** on Windows instead of reusing a normal Ubuntu/Debian development distribution.

```text
Windows desktop
  |
  +-- normal user WSL distributions      <- untouched
  |
  +-- WSL 2 instance: Hacocoon           <- dedicated Hacocoon host
        -> systemd (PID 1)
        -> Incus
           -> Hacocoon Environment

Windows VS Code
  -> haco-vscode
  -> Remote-SSH
  -> Hacocoon Environment
```

Windows and WSL lifecycle remain outside Hacocoon Core. Installation is handled by host-side bootstrap tooling.

## Recommended installer

GitHub Releases publishes `install-windows.ps1` as the standalone Windows installer. It does **not** require a Hacocoon repository checkout.

Download that release asset to Windows and run it from an elevated PowerShell:

```powershell
.\install-windows.ps1
```

The default dedicated instance is `Hacocoon` and the default base distribution is `Ubuntu-26.04`.

The installer is resumable. On a fresh machine it may first create the WSL instance and stop because Windows needs a reboot or the new distribution needs first-launch Linux user creation. In that case:

```powershell
wsl -d Hacocoon
```

complete the Linux user setup, exit, and run `install-windows.ps1` again.

## WSL 2 is required and enforced

New distributions installed through current WSL normally use WSL 2 by default, but Hacocoon does not rely on that default alone.

The installer checks the dedicated Hacocoon instance with `wsl --list --verbose`. If that **dedicated** instance is WSL 1, Hacocoon converts only that instance with:

```powershell
wsl --set-version Hacocoon 2
```

The installer never runs `wsl --set-default-version`, never changes the default WSL distribution, and never converts unrelated user distributions.

This distinction is deliberate: Hacocoon owns the dedicated `Hacocoon` instance, but normal Ubuntu/Debian/Arch instances remain user-owned state.

## systemd is required and enforced

Hacocoon's local Incus path requires systemd to be active as PID 1 inside the dedicated WSL instance.

The Linux bootstrap installs the required packages on apt-based distributions:

```text
systemd
systemd-sysv
```

It then updates only the dedicated instance's `/etc/wsl.conf` so the `[boot]` section contains:

```ini
[boot]
systemd=true
```

Existing unrelated sections and keys in `/etc/wsl.conf` are preserved. Hacocoon does not modify the Windows-global `.wslconfig` file.

If systemd is not active yet, the Linux bootstrap returns a restart-required status to the Windows installer. The installer then restarts **only the dedicated instance**:

```powershell
wsl --terminate Hacocoon
```

and automatically reruns the Linux bootstrap. Installation fails closed if systemd is still not PID 1 after that restart.

The installer also requires a WSL version with supported systemd integration. If `wsl --version` is unavailable, it stops and tells the user to update WSL explicitly with:

```powershell
wsl --update
```

Hacocoon does not silently update WSL itself.

## What the standalone installer does

The installer:

1. validates the requested instance/base/version names;
2. reuses only the named Hacocoon instance if it already exists;
3. otherwise creates a new named WSL instance with `wsl --install <distro> --name <name> --no-launch`;
4. verifies/converts only that dedicated instance to WSL 2;
5. downloads `checksums.txt`, `bootstrap-wsl.sh`, and `install.sh` from the selected Hacocoon release;
6. verifies the downloaded bootstrap scripts with SHA-256 values from the release checksum file;
7. installs systemd support and configures `/etc/wsl.conf`;
8. restarts only the dedicated WSL instance when systemd activation requires it;
9. verifies systemd is PID 1;
10. installs and starts Incus with systemd;
11. installs `haco` and `haco-vscode`.

For a private repository, release download requires an authenticated `gh` CLI or `GH_TOKEN` / `GITHUB_TOKEN`. Public releases can be downloaded directly.

## Dedicated-instance rule

The installer never selects the user's default WSL distribution and never falls back to the first installed distribution.

If `Hacocoon` does not exist, the important platform operation is equivalent to:

```powershell
wsl --install Ubuntu-26.04 --name Hacocoon --no-launch
```

It never automatically unregisters, resets, or deletes another WSL instance; changes the default WSL distribution; changes global WSL defaults; replaces another distribution's Linux user; or modifies Windows `.wslconfig`.

A different base or dedicated instance name may be selected explicitly:

```powershell
.\install-windows.ps1 -BaseDistro Ubuntu
.\install-windows.ps1 -InstanceName Hacocoon-Dev
```

## Hacocoon version

The installer defaults to the latest Hacocoon release. A version can be pinned explicitly:

```powershell
.\install-windows.ps1 -HacocoonVersion v0.8.0
```

The Linux release installer verifies the selected binary archive against `checksums.txt` before installing `haco` and `haco-vscode`.

## Incus authority is explicit

Installing Incus and granting control of the Incus daemon are separate operations.

The installer does **not** silently add the Linux user to `incus-admin`. Local Incus administrator access is effectively root-equivalent because it can attach host paths/devices and alter instance security.

When the workstation owner explicitly accepts that authority:

```powershell
.\install-windows.ps1 -GrantIncusAdmin
```

After group membership changes, restart the WSL shell or use `newgrp incus-admin` before relying on the new membership.

If Incus is managed separately:

```powershell
.\install-windows.ps1 -SkipIncus
```

Systemd is still enforced even when Incus installation is skipped, because WSL 2 + systemd are part of the supported dedicated-host contract.

## Repository bootstrap for development

A source checkout still contains:

```powershell
.\scripts\bootstrap-windows.ps1
```

That path uses the checkout's local `bootstrap-wsl.sh` and `install.sh`, but follows the same WSL 2 and systemd guarantees as the standalone installer.

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

## VS Code

VS Code remains a Windows desktop client. Hacocoon does not add another AI UI.

```text
install-windows.ps1
  -> dedicated Hacocoon WSL 2
  -> systemd
  -> Incus + Hacocoon
  -> workspace in dedicated WSL filesystem
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> /workspace inside the Hacocoon Environment
  -> existing VS Code AI / terminal / Git UI
```

## Acceptance boundary

CI validates PowerShell/shell syntax, the WSL 2/systemd contract, release checksum inclusion, release packaging, and repository integration. It does not prove Windows feature enablement, reboot behavior, named distribution installation, first-run user setup, real WSL 2 conversion, real systemd startup, real Incus startup, or desktop VS Code Remote-SSH.

Those remain real-host acceptance tests and must not be reported as passed until they run on an appropriate Windows + dedicated Hacocoon WSL 2 + systemd + Incus host.
