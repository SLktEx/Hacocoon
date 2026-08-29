# Windows / WSL bootstrap

Hacocoon runs its local runtime on Linux. On a Windows workstation, the supported local shape is:

```text
Windows desktop
  -> WSL 2
     -> Incus
        -> Hacocoon Environment
  -> VS Code desktop
     -> haco-vscode
     -> Remote-SSH
```

The bootstrap scripts make the Windows/WSL host setup repeatable without turning Windows or WSL lifecycle into Hacocoon Core responsibilities.

## Bootstrap entrypoint

From a PowerShell prompt in a Hacocoon checkout:

```powershell
.\scripts\bootstrap-windows.ps1
```

If WSL and a Linux distribution already exist, the script uses the default distribution (or the first installed distribution when no default can be identified).

To select a distribution explicitly:

```powershell
.\scripts\bootstrap-windows.ps1 -Distro Ubuntu
```

The script uses the normal Windows `wsl.exe` management commands. It does not unregister, delete, reset, or overwrite an existing distribution.

If no usable distribution exists, the script installs a WSL 2 distribution and then stops. Windows may require a reboot, and a newly installed distribution may require first-launch Linux user creation. Complete that platform-owned setup and run the bootstrap again.

Microsoft documents the supported distribution names through:

```powershell
wsl --list --online
```

Do not hard-code an assumed Store distribution name when a specific version is required; pass the name returned by the local WSL installation through `-Distro`.

## What is installed inside WSL

For an apt-based distribution, `scripts/bootstrap-wsl.sh` installs the base dependencies required by the release installer:

- CA certificates;
- `curl`;
- `tar`;
- `git`;
- Incus, unless `-SkipIncus` is used.

It then delegates Hacocoon installation to the existing `scripts/install.sh`. The bootstrap does not create a second release/download implementation.

The release installer installs both:

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

When the workstation owner explicitly wants the WSL user to control the local Incus daemon, run:

```powershell
.\scripts\bootstrap-windows.ps1 -GrantIncusAdmin
```

After group membership changes, restart the WSL shell or use `newgrp incus-admin` before relying on the new membership.

If Incus is already managed separately:

```powershell
.\scripts\bootstrap-windows.ps1 -SkipIncus
```

## Systemd and Incus initialization

The bootstrap attempts to start the packaged Incus service when systemd is active. It applies `incus admin init --minimal` only when the daemon is reachable and no storage pool exists.

The bootstrap deliberately does not rewrite `/etc/wsl.conf` or force-enable systemd. If the selected distribution does not run systemd, fix that WSL configuration explicitly and re-run the bootstrap.

## Existing WSL distributions

Existing distributions are treated as user-owned state.

The bootstrap:

- does not unregister them;
- does not reset them;
- does not convert WSL 1 distributions automatically;
- does not change the default distribution;
- does not replace existing Linux users;
- does not rewrite arbitrary WSL configuration.

If the selected distribution is WSL 1, the script fails with an explicit instruction to perform the conversion yourself:

```powershell
wsl --set-version <Distro> 2
```

Automatic conversion is intentionally avoided because it can be expensive and affects user-owned VM state.

## Workspace placement

For the normal local Incus path, keep active Hacocoon workspaces in the WSL Linux filesystem rather than treating a Windows-mounted path under `/mnt/c` as the default workspace location.

Example:

```bash
mkdir -p ~/src
cd ~/src
git clone <repository>
cd <repository>
haco-vscode open .
```

This keeps Linux ownership, filesystem semantics, Incus bind-mount behavior, and developer tooling on the same side of the Windows/WSL boundary.

## VS Code

The bootstrap does not install or replace VS Code's AI UI. VS Code remains a Windows desktop client and `haco-vscode` remains a thin Client Adapter.

After the WSL/Incus setup is ready, the intended workflow is:

```text
PowerShell bootstrap
  -> WSL 2 + Incus + Hacocoon
  -> workspace in WSL filesystem
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> /workspace inside the Hacocoon Environment
  -> existing VS Code AI / terminal / Git UI
```

## Acceptance boundary

CI can validate script syntax and repository integration, but it cannot prove Windows feature enablement, reboot behavior, Store distribution installation, real WSL 2 kernel behavior, real Incus startup, or desktop VS Code Remote-SSH.

Those remain real-host acceptance tests and must not be reported as passed unless they were run on an appropriate Windows + WSL 2 + Incus host.
