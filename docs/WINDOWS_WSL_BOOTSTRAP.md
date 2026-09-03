# Windows / WSL installation

Hacocoon supports Ubuntu 26.04+ as its local Host baseline. On Windows it creates a dedicated Ubuntu WSL 2 distribution; on native Ubuntu it uses the host directly.

The installer deliberately uses a **pre / main / post** split rather than pretending WSL and native Ubuntu are identical.

## Installation phases

```text
Windows / WSL
install-windows.bat
  -> install-windows.ps1 pre
  -> install.sh             common Ubuntu main
  -> install-windows.ps1 post

native Ubuntu
install-ubuntu.sh pre
  -> install.sh             common Ubuntu main
  -> install-ubuntu.sh post
```

`install.sh` contains only work shared by Ubuntu on both substrates: common packages, Incus, the Hacocoon binaries, the Physical Host controller, trusted `haco-host` reconciliation, and the controller round-trip acceptance check.

WSL lifecycle work does not belong in `install.sh`. Native-Ubuntu-only policy does not belong there either.

## Architecture-specific packages

Releases build separate packages for each CPU architecture. A package never carries binaries for the other architecture.

```text
hacocoon-windows-amd64.zip
├─ install-windows.bat
├─ install-windows.ps1
├─ install.sh
├─ haco_linux_amd64.tar.gz
├─ checksums.txt
└─ VERSION

hacocoon-windows-arm64.zip
├─ install-windows.bat
├─ install-windows.ps1
├─ install.sh
├─ haco_linux_arm64.tar.gz
├─ checksums.txt
└─ VERSION

hacocoon-ubuntu-amd64.tar.gz
├─ install-ubuntu.sh
├─ install.sh
├─ haco_linux_amd64.tar.gz
├─ checksums.txt
└─ VERSION

hacocoon-ubuntu-arm64.tar.gz
├─ install-ubuntu.sh
├─ install.sh
├─ haco_linux_arm64.tar.gz
├─ checksums.txt
└─ VERSION
```

The normal installation path therefore does **not** download `install.sh` or the Hacocoon binary archive again. The archive that is installed is the archive bundled with the installer package.

Network access can still be required for Ubuntu packages and for provenance verification. That is separate from downloading a second copy of the Hacocoon release payload.

The raw `haco_linux_amd64.tar.gz` and `haco_linux_arm64.tar.gz` release assets remain available for advanced/standalone use, but they are not the normal Host-installation entry point.

## Windows pre phase

The PowerShell installer owns Windows and WSL-specific preparation:

1. require a current WSL installation;
2. create or reuse only the dedicated `Hacocoon` distribution from `Ubuntu-26.04`;
3. enforce WSL 2 for that distribution without changing global WSL defaults;
4. on a fresh/default setup, create the dedicated non-root Ubuntu user `hacocoon` automatically;
5. optionally use Ubuntu's normal first-launch user setup when `-InteractiveUserSetup` is selected;
6. verify the WSL guest is Ubuntu 26.04+;
7. preserve unrelated `/etc/wsl.conf` settings while ensuring the selected login user and systemd are configured:

   ```ini
   [user]
   default=hacocoon

   [boot]
   systemd=true
   ```

8. restart only the dedicated distribution with `wsl --terminate Hacocoon` when systemd activation requires it;
9. verify systemd is PID 1;
10. verify that the installer package contains the binary archive matching the WSL architecture.

The default fresh-install path is one-shot: running `install-windows.bat` creates the WSL distribution, creates the `hacocoon` user, enables systemd, runs the packaged `install.sh`, performs WSL post-install integration, and returns only after the installation is complete. The user does not need to launch WSL and rerun the BAT between phases.

For an interactive Ubuntu account setup instead, run:

```powershell
.\install-windows.bat -InteractiveUserSetup
```

The installer launches the normal Ubuntu first-launch setup. After creating the Ubuntu account and reaching its shell, run `exit` once; the still-running Windows installer then resumes automatically and finishes `install.sh` plus post-install integration. An existing dedicated distribution with a normal non-root default user is reused on ordinary reruns.

The automatic bootstrap user receives a temporary passwordless bootstrap sudo rule only while the common installation is running. That broad temporary rule is removed in a `finally` path. Normal completed WSL integration retains only the exact passwordless `haco host ensure` and `haco host shell` commands described below.

## Common Ubuntu main phase

Both Windows/WSL and native Ubuntu invoke the same packaged `install.sh`.

The common main phase:

- requires Ubuntu 26.04+ and an already-running systemd;
- installs common Host dependencies;
- installs and initializes Incus unless `-SkipIncus` / its environment equivalent is selected;
- verifies the bundled architecture-specific archive checksum;
- verifies trusted GitHub/Sigstore provenance and signed release binding when provenance is enabled;
- validates the archive contains exactly the expected regular Hacocoon binaries;
- installs the binaries and root-owned storage helper;
- installs/restarts `haco-controller.service`;
- requires `/run/hacocoon/control.sock` to be a root-owned mode `0600` Unix socket;
- runs `haco host ensure`;
- proves the real trusted-host path with `/usr/local/bin/haco-host doctor` inside `haco-host`.

`install.sh` does not edit `/etc/wsl.conf`, terminate WSL, or change a user's login shell.

## Windows post phase

After the common main phase succeeds, PowerShell performs WSL-only integration.

It validates the system-owned `haco` binary, creates `/usr/local/libexec/hacocoon-login`, grants passwordless sudo only for the exact `haco host ensure` and `haco host shell` commands, and changes only the normal non-root WSL user's login shell.

After that:

```powershell
wsl -d Hacocoon
```

enters the trusted `haco-host` management environment. Explicit/non-interactive WSL commands remain on the Physical Host.

The root user's shell is never replaced. Recovery remains:

```powershell
wsl -d Hacocoon -u root
```

The raw Incus socket is not exposed to `haco-host`.

## Native Ubuntu pre and post

The native package entry point is:

```bash
./install-ubuntu.sh
```

Its pre phase rejects WSL, verifies Ubuntu 26.04+, requires systemd as PID 1, and verifies sudo availability when needed. It then invokes the same packaged `install.sh`.

Its post phase intentionally leaves the native Ubuntu user's login shell unchanged. Enter the trusted Host explicitly with:

```bash
haco host shell
```

## E2E acceptance boundary

Installer E2E is evaluated at the user-visible entry points, not by declaring success because `install.sh` ran in isolation.

The Windows default-path gate builds the candidate `hacocoon-windows-amd64.zip`, extracts it, executes the packaged `install-windows.bat` once, and requires that single invocation to create the dedicated WSL user and proceed through WSL 2, systemd, the common `install.sh`, Incus, the controller socket/service, `haco-host doctor`, and WSL login integration. Interactive user setup is a separate opt-in path, not a prerequisite for the default installer.

The native Ubuntu gate builds the candidate `hacocoon-ubuntu-amd64.tar.gz`, extracts it, executes the packaged `install-ubuntu.sh`, and requires the controller and trusted `haco-host` round trip to succeed while confirming the native login shell was not replaced.

PR candidate packages are not public releases and therefore do not yet have release attestations. Candidate E2E can disable provenance only for that synthetic package; the release workflow independently signs and attests the exact architecture-specific payload that is published.
