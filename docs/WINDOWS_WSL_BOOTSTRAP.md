# Windows / WSL installation

Hacocoon uses a **dedicated WSL 2 distribution with systemd** on Windows instead of reusing a normal development distribution.

That distribution is the **Physical Host**. It owns Incus, managed Btrfs primitives, and the Hacocoon controller. After installation succeeds, normal interactive entry goes directly into the persistent trusted `haco-host` instance.

```text
Windows
  |
  +-- normal WSL distributions                 <- untouched
  |
  `-- WSL 2: Hacocoon                          <- Physical Host
       |- systemd (PID 1)
       |- Incus
       |- loop / Btrfs primitives
       |- haco-controller
       |    `- /run/hacocoon/control.sock
       |
       `- Incus: haco-host                     <- TRUSTED default Host
            |- /usr/local/bin/haco-host
            `- /var/lib/hacocoon-control.sock
                 `- dedicated Incus proxy to controller

       Incus: managed Environments              <- UNTRUSTED
```

See [`design/trusted-host.md`](design/trusted-host.md) and [`design/controller-client-transport.md`](design/controller-client-transport.md).

## One Linux installer path

Linux and WSL use the same `scripts/install.sh` implementation for Host setup and Hacocoon installation.

```text
native Linux ---------------------> install.sh
                                      |
Windows -> install-windows.ps1 -> WSL -> install.sh
```

`install.sh` detects WSL. Common work such as dependency preparation, Incus setup, release verification, binary installation, controller setup, `haco-host` reconciliation, and the controller round-trip is shared. Only WSL-specific behavior is conditional: `/etc/wsl.conf` systemd activation, the restart-required exit code, and default WSL entry into `haco-host`.

Native Linux does not modify the user's login shell. It uses the same Host bootstrap and then leaves `haco host shell` as an explicit operation.

## Normal Windows installer

GitHub Releases publish **`hacocoon-windows-installer.zip` as the normal Windows installer**. A repository checkout is not required.

Extracting the ZIP gives:

```text
hacocoon-windows-installer/
├─ install-windows.bat
├─ install-windows.ps1
└─ install.sh
```

The ZIP therefore already carries the Linux installer that PowerShell will execute inside the dedicated WSL distribution. The normal Windows path does not download another copy of the installer script during installation. `install.sh` downloads only the selected Hacocoon release archive and its verification inputs.

Normally, right-click `install-windows.bat` and choose **Run as administrator**, or run it from an elevated Command Prompt:

```bat
install-windows.bat
```

The BAT launcher invokes only the sibling `install-windows.ps1` with:

```text
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File ...
```

`-ExecutionPolicy Bypass` is supplied only to that PowerShell process. The launcher never calls `Set-ExecutionPolicy` and therefore does not persistently change machine or user policy. Installer arguments and the installer exit code are forwarded unchanged.

This keeps direct execution of an Internet-downloaded `.ps1` out of the normal path, where PowerShell Execution Policy and Mark-of-the-Web commonly add friction. It does **not** disable organization-managed `MachinePolicy` / `UserPolicy`, Windows reputation protection, or other endpoint policy; environments that prohibit the operation still fail closed.

The standalone `install-windows.ps1` asset remains available for advanced use and compatibility. When its sibling `install.sh` is unavailable, it resolves the requested release, downloads `install.sh`, and verifies the standalone asset before execution. Direct PowerShell invocation remains subject to the machine's configured execution policy.

The default WSL distribution is `Hacocoon`; the default base is `Ubuntu-26.04`.

A fresh distribution may first require normal Linux user creation. Before Hacocoon installation has completed, `wsl -d Hacocoon` still enters that base distribution for first-run setup. Run `install-windows.bat` as administrator again afterwards.

Once installation succeeds:

```powershell
wsl -d Hacocoon
```

enters trusted `haco-host`.

## WSL 2 and systemd

The installer verifies that only the dedicated Hacocoon distribution is WSL 2 and uses `wsl --set-version Hacocoon 2` when necessary. It does not change the global WSL default or unrelated distributions.

The shared Linux installer preserves unrelated `/etc/wsl.conf` settings while ensuring:

```ini
[boot]
systemd=true
```

If systemd is not yet PID 1, the Windows installer terminates only the Hacocoon distribution with `wsl --terminate Hacocoon` and retries. It does not use `wsl --shutdown`.

## Incus and managed storage

Unless `-SkipIncus` is selected, installation installs/starts Incus and initializes it minimally when required.

The default local storage backend is Hacocoon-managed sparse-raw Btrfs mounted with `compress=zstd:3`. `compress-force` and automatic defrag/recompression are intentionally avoided because they can damage useful reflink/COW sharing.

## Physical Host controller service

After installing the release, `install.sh` validates that `haco-controller` is a root-owned, non-group/world-writable system binary at `/usr/local/bin` or `/usr/bin`.

It then installs and restarts:

```text
haco-controller.service
```

on the Physical Host. The unit uses a private systemd runtime directory and the controller listens only on:

```text
/run/hacocoon/control.sock
```

Before continuing, installation requires that path to be a Unix socket owned by `root:root` with mode `0600`.

The supported local path does not open a localhost TCP listener.

Restarting the service during installation is intentional: after an upgrade, the controller process must run the same release that was just installed instead of retaining an old binary in memory.

## Trusted `haco-host` reconciliation

With the controller active, installation runs Physical-Host-authority:

```text
haco host ensure
```

That operation reconciles:

- exactly one Incus instance named `haco-host`;
- `user.hacocoon.role=trusted-host` ownership;
- Hacocoon-managed root storage;
- automatic restart of an owned stopped instance;
- `environment.HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock`;
- the dedicated `haco-control` proxy;
- client-only `/usr/local/bin/haco-host` inside the trusted instance.

The proxy is intentionally narrow:

```text
type=proxy
bind=instance
listen=unix:/var/lib/hacocoon-control.sock
connect=unix:/run/hacocoon/control.sock
mode=0600
uid=0
gid=0
```

An unexpected existing instance, endpoint variable, or proxy shape is rejected instead of silently repurposed.

The client binary is compared by SHA-256 and final root ownership/mode before provisioning is considered complete.

## Installation proves the round trip

After `haco host ensure`, `install.sh` executes inside the actual trusted instance:

```text
/usr/local/bin/haco-host doctor
```

This must complete the real path:

```text
haco-host CLI
  -> /var/lib/hacocoon-control.sock
  -> Incus haco-control proxy
  -> /run/hacocoon/control.sock
  -> Physical Host haco-controller
```

If this fails, installation stops before changing the normal WSL user's login shell and prints the Physical Host recovery path.

The raw Incus daemon socket is never mounted or proxied into `haco-host`.

## Why `wsl -d Hacocoon` enters `haco-host`

After controller/Host acceptance succeeds, the WSL branch of `install.sh` creates root-owned `/usr/local/libexec/hacocoon-login` and makes it the normal non-root WSL user's login shell.

For interactive no-command entry, that invocation delegates to:

```text
sudo -n <system-owned-haco> host shell
```

`haco host shell` re-reconciles the trusted Host and client binary, prints the privileged-management warning, then enters `/bin/bash -l` inside `haco-host`.

Explicit/non-interactive WSL commands remain on the Physical Host and do not receive the interactive warning.

## Automatic-entry privilege boundary

Automatic entry does not require `incus-admin` for the normal WSL user.

The installer grants passwordless sudo only for the exact system-owned commands:

```text
haco host ensure
haco host shell
```

The raw Incus socket and `/var/lib/incus` remain Physical Host authority.

Operators who intentionally want root-equivalent local Incus authority can use:

```bat
install-windows.bat -GrantIncusAdmin
```

The standalone `./install-windows.ps1 -GrantIncusAdmin` path remains available as well.

## Physical Host recovery

Normal use:

```powershell
wsl -d Hacocoon
```

enters `haco-host`.

The root account's shell is never replaced. Direct Physical Host recovery remains:

```powershell
wsl -d Hacocoon -u root
```

Explicit commands can also target the Physical Host, for example:

```powershell
wsl -d Hacocoon -- haco status
```

Operations requiring Physical Host root authority still need an authorized sudo path or the root recovery shell.

## `-SkipIncus`

For deployments where Incus is managed separately, use:

```bat
install-windows.bat -SkipIncus
```

The standalone `./install-windows.ps1 -SkipIncus` path remains available as well.

In this mode installation does not claim that the trusted backend is ready, so it leaves the Physical Host login unchanged and does not configure the controller-connected automatic `haco-host` entry.

## Workspace location

Default interactive entry and Workspace ownership are separate architecture seams.

Moving repository/workspace ownership fully into the logical Host is not implied by this installer. Until that work lands, Physical Host paths can still be targeted explicitly, and VS Code/external orchestration should use Hacocoon's client/control surface rather than treating `haco-host` as a mandatory SSH jump host.

## Installer sequence

The supported Windows path now performs, in order:

1. validate the named WSL distribution and requested release;
2. create/reuse only the Hacocoon distribution;
3. enforce WSL 2 for that distribution;
4. invoke the same `install.sh` used by native Linux;
5. enable systemd and restart only that distribution when required;
6. install/start Incus unless skipped;
7. verify and install the Hacocoon release binaries;
8. install/restart the Physical Host `haco-controller.service`;
9. verify the root-only controller Unix socket;
10. reconcile trusted `haco-host`, its narrow proxy, and client binary;
11. prove `haco-host doctor` reaches the Physical Host controller;
12. install the narrow automatic-entry sudo rule;
13. switch the normal non-root WSL user's login shell to `hacocoon-login`.

Global WSL defaults, `.wslconfig`, unrelated distributions, and the root user's login shell are not modified.

## Checkout/developer bootstrap

A repository checkout can still use:

```powershell
.\scripts\bootstrap-windows.ps1
```

It is only the Windows/WSL wrapper for a source checkout and invokes the same `scripts/install.sh` used by native Linux and the packaged installer.

## Release integrity

`hacocoon-windows-installer.zip` is covered by the Release SHA-256 checksum and GitHub artifact attestations. CI verifies that the ZIP contains exactly `install-windows.bat`, `install-windows.ps1`, and `install.sh`, and that all three members are byte-for-byte identical to their source files.

The normal ZIP path executes its bundled `install.sh`; it does not fetch a second installer script from the Release. The shared Linux installer resolves `latest` to an explicit tag, downloads the selected Linux archive and `checksums.txt`, and verifies SHA-256 plus trusted GitHub/Sigstore provenance and signed release binding before installing binaries.

The standalone `install-windows.ps1` compatibility path additionally verifies a separately downloaded `install.sh` before executing it when no sibling installer is present.

## Acceptance boundary

Repository CI and real Incus E2E can prove the controller protocol, real proxy device, client provisioning, `haco-host doctor` round trip, restart recovery, raw Incus-socket non-exposure, ordinary Environment endpoint isolation, and the BAT launcher/ZIP packaging contract.

The Windows all-scripts E2E builds the candidate ZIP from the pull request, extracts it, and executes the packaged BAT/PowerShell path before exercising the shared `install.sh` directly inside WSL. Release publication itself remains a separate post-merge gate because a pull request does not publish a new public Release.
