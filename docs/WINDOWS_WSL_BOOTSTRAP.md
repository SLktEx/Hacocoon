# Windows / WSL installation

Hacocoon supports Ubuntu 26.04+ as its local Host baseline. On Windows it creates a dedicated Ubuntu WSL 2 distribution; on native Ubuntu it uses the host directly.

The installer deliberately uses a **pre / main / post** split rather than pretending WSL and native Ubuntu are identical.

## Installation phases

Current status: **partial**. The reset product `haco` exposes help/version and WSL login; retained lifecycle commands currently belong to temporary `hacoq`. See [CLI migration](CLI_MIGRATION.md). One-invocation BAT continuation and root-side preparation are implemented; real-host completion, network and restart/rerun acceptance remain pending. The common phase ships no privileged storage executable; Incus alone owns the Btrfs lifecycle. [ADR 0004](adr/0004-wsl-installer-authority.md) owns installer authority.

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

A fresh or interrupted setup opens the normal Ubuntu first-launch session inside the same BAT invocation. Complete the OS prompts, including account creation and any metrics-consent choice, then type `exit` at the initial Linux shell. The BAT continues automatically. Only a completed current installation with the Hacocoon login shell skips this initial session. Existing users and data are preserved; a failed or interrupted setup retains the distribution for another run of the current BAT.

## Common Ubuntu main phase

Both Windows/WSL and native Ubuntu invoke the same packaged `install.sh`.

Windows invokes it as WSL root and passes the ordinary login name through `HACO_INSTALL_USER`. The common phase validates that account and resolves its actual non-root UID/GID before host preparation; these exact IDs drive Incus subordinate-ID mapping. Privileged execution must not substitute root for the workspace owner. Native user/sudo invocation retains its ordinary caller identity.

The common main phase:

- requires Ubuntu 26.04+ and an already-running systemd;
- installs common Host dependencies;
- installs and initializes Incus unless `-SkipIncus` / its environment equivalent is selected;
- verifies the bundled architecture-specific archive checksum;
- verifies trusted GitHub/Sigstore provenance and signed release binding when provenance is enabled;
- validates the archive contains exactly the expected regular Hacocoon binaries;
- installs the Hacocoon binaries;
- installs/restarts `haco-controller.service`;
- adds the ordinary user to the privileged local `hacocoon` controller access group;
- requires `/run/hacocoon/control.sock` to be a `root:hacocoon` mode `0660` Unix socket;
- internally runs retained `hacoq host ensure` during the CLI migration;
- proves the real trusted-host path with `/usr/local/bin/haco-host doctor` inside `haco-host`.

`install.sh` does not edit `/etc/wsl.conf`, terminate WSL, or change a user's login shell.

## Windows post phase

After the common main phase succeeds, PowerShell performs WSL-only integration.

It validates the system-owned `haco` binary, creates its `/usr/local/libexec/hacocoon-login` alias, and changes only the normal non-root WSL user's login shell. The alias uses the controller directly. The installer writes no bootstrap or login sudo policy, and does not grant `incus-admin` unless explicitly requested.

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
hacoq host shell
```

## E2E acceptance boundary

Installer E2E is evaluated at the user-visible entry points, not by declaring success because `install.sh` ran in isolation.

The Windows gate builds the candidate `hacocoon-windows-amd64.zip`, extracts it, types the unchanged BAT in a normal terminal, and answers Ubuntu's actual first-launch dialogs. It requires completion before any second BAT. It then enters `wsl -d Hacocoon`, checks the implemented product help/version, writes a trusted-host file, stops and reopens WSL, and only then reruns the same BAT and checks file and sudo-policy preservation. Root assertions are read-only. No product overrides, account/sudoers fixtures, test bridge, or mount repairs may fill product gaps.

This checks trusted-host file retention, not Environment/Workspace work retention. The new CLI's lifecycle/SSH journey, installer-created network DNS/routes/HTTPS, and allowed proxy versus denied direct Environment traffic remain separate required acceptance. Linux Incus/network foundation CI continues; the removed Windows legacy fixture journey is not evidence for the new product path.

The native Ubuntu gate builds the candidate `hacocoon-ubuntu-amd64.tar.gz`, extracts it, executes the packaged `install-ubuntu.sh`, and requires the controller and trusted `haco-host` round trip to succeed while confirming the native login shell was not replaced.

PR candidate packages are not public releases. The bundled-payload path verifies its checksum without requiring a product environment override; the release distribution workflow independently signs and attests the exact package it publishes. Standalone downloaded payloads still require provenance verification by default.
