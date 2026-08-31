# Windows / WSL installation

Hacocoon uses a **dedicated Ubuntu 26.04 WSL 2 distribution with systemd** on Windows instead of reusing a normal development distribution.

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

## Pre / main / post installation model

WSL Ubuntu and native Ubuntu deliberately share only the work that is actually common.

```text
Windows / WSL
  install-windows.ps1 pre
        |
        v
     install.sh              <- Ubuntu-common main
        |
        v
  install-windows.ps1 post

native Ubuntu
  install-ubuntu.sh pre
        |
        v
     install.sh              <- same Ubuntu-common main
        |
        v
  install-ubuntu.sh post
```

The split is intentional:

- **WSL pre** owns WSL installation, WSL 2, Ubuntu baseline validation, systemd configuration in `/etc/wsl.conf`, and the targeted WSL restart needed to activate systemd.
- **Ubuntu-common main** in `install.sh` owns Ubuntu packages, GitHub CLI trust bootstrap, Incus, release download and verification, Hacocoon binaries, controller service, `haco-host` reconciliation, and the controller round trip.
- **WSL post** owns the WSL login-shell integration and optional `incus-admin` grant for the normal WSL user.
- **native Ubuntu pre/post** stay in `install-ubuntu.sh`; native Ubuntu never receives WSL-specific configuration or login-shell replacement.

`install.sh` does not detect WSL and does not edit `/etc/wsl.conf`.

## Normal Windows installer

GitHub Releases publish **`hacocoon-windows-installer.zip` as the normal Windows installer**. A repository checkout is not required.

Extracting the ZIP gives:

```text
hacocoon-windows-installer/
├─ install-windows.bat
├─ install-windows.ps1
└─ install.sh
```

The ZIP already carries the shared Ubuntu installer that PowerShell executes inside the dedicated WSL distribution. The normal Windows path therefore does **not** download another copy of `install.sh` during installation. `install.sh` downloads only the selected Hacocoon Linux release archive and the verification material needed for that archive.

Normally, right-click `install-windows.bat` and choose **Run as administrator**, or run it from an elevated Command Prompt:

```bat
install-windows.bat
```

The BAT launcher invokes only the sibling `install-windows.ps1` with a process-local execution-policy bypass. It never changes the machine or user PowerShell policy.

The default WSL distribution is `Hacocoon`; the supported base is `Ubuntu-26.04`.

## First run and Ubuntu user setup

On a fresh machine the first installer run creates the dedicated WSL distribution and then stops before Hacocoon setup so normal Ubuntu first-run user creation can occur.

```powershell
wsl -d Hacocoon
```

Complete normal Ubuntu user setup, exit the distribution, then run `install-windows.bat` again. The second run performs WSL pre, the shared `install.sh`, and WSL post.

Once installation succeeds:

```powershell
wsl -d Hacocoon
```

enters trusted `haco-host`.

## WSL pre: WSL 2 and systemd

The Windows installer verifies that only the dedicated Hacocoon distribution is WSL 2 and uses `wsl --set-version Hacocoon 2` when necessary. It does not change the global WSL default or unrelated distributions.

PowerShell preserves unrelated `/etc/wsl.conf` settings while ensuring:

```ini
[boot]
systemd=true
```

When activation requires a restart, PowerShell terminates only the Hacocoon distribution with:

```text
wsl --terminate Hacocoon
```

It does not use `wsl --shutdown`.

Before calling the shared installer, WSL pre requires Ubuntu 26.04 or newer and requires systemd to be active as PID 1.

## Ubuntu-common main: `install.sh`

The shared installer is the same script used by native Ubuntu. It:

1. validates Ubuntu 26.04+ and active systemd;
2. installs common host dependencies;
3. ensures an attestation-capable GitHub CLI using GitHub CLI's signed APT repository when necessary;
4. installs and starts Incus unless explicitly skipped;
5. resolves the requested Hacocoon release;
6. downloads the matching Linux archive and `checksums.txt`;
7. verifies SHA-256, GitHub/Sigstore build provenance, and the signed release/tag binding;
8. installs the Hacocoon binaries and root-owned storage helper;
9. installs/restarts `haco-controller.service`;
10. requires the controller socket to be `root:root` mode `0600`;
11. reconciles the trusted `haco-host` instance;
12. proves the real `haco-host -> controller` round trip with `haco-host doctor`.

Public installation does not require a GitHub login. When `gh` is unauthenticated, `install.sh` retrieves public attestation bundles through GitHub's public API and verifies them locally with `gh attestation verify --bundle` while retaining the repository, workflow, source-ref, self-hosted-runner, and release-binding constraints.

## WSL post: trusted default entry

After the common main phase succeeds, PowerShell performs only WSL-specific integration.

It verifies the installed system `haco` binary is root-owned and not group/world writable, installs `/usr/local/libexec/hacocoon-login`, adds only the narrow passwordless sudo commands:

```text
haco host ensure
haco host shell
```

and changes only the normal non-root WSL user's login shell to the Hacocoon entry point.

The raw Incus socket and `/var/lib/incus` remain Physical Host authority. Automatic entry does not require `incus-admin`.

Operators who intentionally want root-equivalent local Incus authority can use:

```bat
install-windows.bat -GrantIncusAdmin
```

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

Explicit non-interactive commands can also target the Physical Host.

## `-SkipIncus`

For deployments where Incus is managed separately:

```bat
install-windows.bat -SkipIncus
```

In this mode the common main phase installs Hacocoon binaries but does not claim the trusted Incus backend is ready, and WSL post leaves automatic `haco-host` entry unconfigured.

## Developer checkout wrapper

A source checkout can still use:

```powershell
.\scripts\bootstrap-windows.ps1
```

This is now only a thin compatibility/developer wrapper around `scripts/install-windows.ps1`; it contains no second implementation of the installation lifecycle.

## Release integrity

`hacocoon-windows-installer.zip` is covered by the Release checksum and GitHub artifact attestations. CI verifies that it contains exactly `install-windows.bat`, `install-windows.ps1`, and `install.sh`, byte-for-byte matching the source files used for packaging.

The package is the installer unit. The bundled `install.sh` is not fetched again from the Release during the Windows flow.

## E2E acceptance boundary

The Windows installer E2E evaluates the actual candidate user path:

1. build and extract the candidate Windows installer ZIP;
2. run the packaged `install-windows.bat` to create the dedicated WSL distribution;
3. emulate completion of normal Ubuntu first-user setup;
4. run the **same packaged BAT** again;
5. require systemd, Incus, `haco-controller.service`, trusted `haco-host`, the controller round trip, and the WSL login integration to be ready.

Direct execution of internal `install.sh` or `install-windows.ps1` is not used as an E2E success substitute. Release-page publication itself remains a post-merge/release concern rather than a pull-request prerequisite.
