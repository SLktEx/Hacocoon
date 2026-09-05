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
4. on a fresh default install, create the managed non-root `hacocoon` user, lock password login for that account, and configure it as the WSL default user;
5. optionally use Ubuntu's interactive user setup instead when `-InteractiveUserSetup` is specified;
6. preserve an already-configured non-root default user when upgrading an older installation;
7. verify the WSL guest is Ubuntu 26.04+;
8. preserve unrelated `/etc/wsl.conf` settings while ensuring the managed/default user and:

   ```ini
   [boot]
   systemd=true
   ```

9. restart only the dedicated distribution with `wsl --terminate Hacocoon` when login-user or systemd activation changes require it;
10. verify systemd is PID 1;
11. verify that the installer package contains the binary archive matching the WSL architecture.

A fresh default installation is intentionally **one shot**:

```powershell
install-windows.bat
```

creates the `Hacocoon` WSL distribution, creates the managed `hacocoon` user, configures systemd, runs the packaged `install.sh`, performs WSL post-integration, and returns only after the installation is complete. The user does not have to launch WSL and run the BAT a second time.

For users who explicitly want Ubuntu's normal interactive account creation flow, run:

```powershell
install-windows.bat -InteractiveUserSetup
```

The installer launches the WSL user-setup session itself. After the user completes setup and exits that shell, the same installer process resumes and continues through `install.sh` and the post phase; a second BAT invocation is still not required.

## Cached WSL image validation path

`-UseCachedWslImage` is a validation-oriented installer option for repeated Windows/WSL installation tests. It keeps the normal installer behavior unchanged unless the option is explicitly selected.

When enabled, `install-windows.ps1` uses `ubuntu.wsl` next to the installer package as the local Ubuntu 26.04 image cache. If that file is absent, the installer reads Microsoft's WSL `DistributionInfo.json`, resolves the `Ubuntu-26.04` image for the current Windows architecture, downloads it to a temporary file, verifies the published SHA256, and only then promotes it to `ubuntu.wsl`.

The dedicated distribution is then created with the named-install path:

```powershell
wsl --install --from-file .\ubuntu.wsl --name Hacocoon --no-launch
```

`-UseCachedWslImage` currently supports only the `Ubuntu-26.04` base distribution and cannot be combined with `-WebDownload`. The cache file is intentionally **not** bundled into release installer packages; it is a local/CI acceleration artifact.

GitHub Actions keeps the cache trust boundary separate from untrusted pull requests. A trusted `windows-wsl-image-cache` workflow on `main` owns cache creation with `actions/cache`: on a miss it invokes the same `-UseCachedWslImage` path, so the file is downloaded through Microsoft's metadata and SHA256 validation before it is stored. The pull-request Windows installer E2E uses only `actions/cache/restore`, copies the trusted cached `ubuntu.wsl` into the extracted candidate package when available, and never writes cache state from a PR. If no trusted cache exists, the candidate installer simply performs its normal verified download for that run.

The restart/reinstall Windows E2E invokes both packaged BAT installs with `-UseCachedWslImage`, so the cached path itself remains exercised while also proving `wsl --terminate Hacocoon` restart persistence and reinstall idempotency.

During the common installer run, PowerShell gives the selected ordinary WSL user a temporary passwordless sudo rule so `install.sh` can remain the ordinary workspace owner. On Ubuntu 26.04 the rule uses sudo-rs-compatible default-root syntax, and the installer validates the effective sudo policy and proves a non-interactive sudo command succeeds before starting `install.sh`. That broad bootstrap rule exists only for the trusted installer invocation and is removed in `finally`; normal completed installations retain only the narrow `haco host ensure` / `haco host shell` rule described below.

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

The Windows gate builds the candidate `hacocoon-windows-amd64.zip`, extracts it, restores and stages the trusted `ubuntu.wsl` cache when available, and executes the packaged installer through the shipped `-UseCachedWslImage` option. It requires the first install to complete with the managed `hacocoon` user, then explicitly terminates the WSL distribution, proves the existing Environment survives restart before any repair install can run, and finally proves reinstall/idempotency. The acceptance boundary includes WSL 2, systemd, Incus, the controller socket/service, `haco-host doctor`, and WSL login integration. Separate coverage keeps the opt-in interactive user-setup path available without making it the default install contract.

The native Ubuntu gate builds the candidate `hacocoon-ubuntu-amd64.tar.gz`, extracts it, executes the packaged `install-ubuntu.sh`, and requires the controller and trusted `haco-host` round trip to succeed while confirming the native login shell was not replaced.

PR candidate packages are not public releases and therefore do not yet have release attestations. Candidate E2E can disable provenance only for that synthetic package; the release workflow independently signs and attests the exact architecture-specific payload that is published.