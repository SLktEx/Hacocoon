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
4. require a normal non-root Ubuntu user;
5. verify the WSL guest is Ubuntu 26.04+;
6. preserve unrelated `/etc/wsl.conf` settings while ensuring:

   ```ini
   [boot]
   systemd=true
   ```

7. restart only the dedicated distribution with `wsl --terminate Hacocoon` when systemd activation requires it;
8. verify systemd is PID 1;
9. verify that the installer package contains the binary archive matching the WSL architecture.

A freshly created Ubuntu WSL distribution can require its normal first-launch user setup. In that case the installer stops after creation and asks the user to run:

```powershell
wsl -d Hacocoon
```

After completing the Ubuntu user setup, run `install-windows.bat` again.

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

The Windows gate builds and extracts the candidate `hacocoon-windows-amd64.zip` before the product-driving path begins. Every action intended to make Hacocoon do work then uses the same command and interaction a real user uses. The gate launches `install-windows.bat` without Hacocoon-specific arguments or environment overrides, completes normal Ubuntu first-launch through the documented `wsl -d Hacocoon` interactive session, runs the same packaged BAT again, enters the installed Host through the same `wsl -d Hacocoon` command, exercises normal `haco` commands, reruns the unchanged BAT against existing state, and verifies the existing Environment and Workspace remain usable.

The product-driving path must not inject `HACO_*` variables, installer-only E2E arguments/options, CI-specific users or sudoers rules, root preparation, Incus/mount/loop repair, or any other mutation whose purpose is to make a subsequent installer/Hacocoon action succeed. CI may automate ordinary terminal input, but it must not change the command line or product configuration seen by those actions.

Read-only assertions are explicitly allowed between product actions. The E2E may use direct inspection commands such as `systemctl`, `incus storage get/show`, `findmnt`, `losetup`, `stat`, and read-only Environment inspection to prove that the preceding action configured the system correctly. Those assertions may run with the privilege needed to observe state, but they must not mutate or repair state that a later product action depends on. In particular, the reinstall acceptance records and verifies the managed Btrfs mount, zstd compression, loop backing, Incus storage source, controller service, and live Environment before and after rerunning the unchanged BAT.

The native Ubuntu gate builds the candidate `hacocoon-ubuntu-amd64.tar.gz`, extracts it, executes the packaged `install-ubuntu.sh`, and requires the controller and trusted `haco-host` round trip to succeed while confirming the native login shell was not replaced.

PR candidate packages are not public releases and therefore do not yet have release attestations. The release workflow independently signs and attests the exact architecture-specific payload that is published.
