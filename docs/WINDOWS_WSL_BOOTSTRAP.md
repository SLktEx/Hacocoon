# Windows / WSL installation

Status: **partial**. Managed-account bootstrap is implemented; real Windows install/network/restart acceptance is tracked separately in [implementation status](IMPLEMENTATION_STATUS.md). Product `haco` provides controller-backed setup, diagnostics, managed repository/Workspace preparation, Environment SSH/stop and Git approvals. Follow the [managed repository workflow](reference/managed-repository-workflow.md) after installation. Remaining legacy commands belong to temporary `hacoq` during [CLI migration](CLI_MIGRATION.md).

Packaged acceptance, exact commits and unresolved startup failures are tracked in [implementation status](IMPLEMENTATION_STATUS.md). A later successful run does not erase an earlier unexplained failure.

The common phase checks daemon readiness without minimal initialization. The adapter owns the Btrfs pool and trusted-host bridge; fresh installs do not create an unused default directory pool. Existing pools are preserved. Current owned default-profile hosts receive a bounded NIC transition without data deletion. See [trusted networking](design/trusted-host.md#dedicated-trusted-host-network) and [ADR 0005](adr/0005-trusted-host-network-ownership.md).

Hacocoon supports Ubuntu 26.04+ as its local Host baseline. On Windows it creates a dedicated Ubuntu WSL 2 distribution; on native Ubuntu it uses the host directly.

The installer deliberately uses a **pre / main / post** split rather than pretending WSL and native Ubuntu are identical.

The installer retries the same read-only WSL account lookup at most three times, with 250 ms between failed probes; it never retries account/setup mutations through this path. Persistent failure stops the installer and reports the final native exit code. It does not establish that an existing account is missing; inspect WSL execution before changing accounts or data. Candidate `63fdf24` encountered this failure before common setup while the existing account remained intact; the underlying intermittent WSL failure is unconfirmed.

The common installer runs `haco doctor` before its completion message. Configured storage and the live Incus-owned Btrfs mount must both pass; a pending mount policy or other failed check stops with a next action. Repeating the current installer does not bypass readiness.

## Diagnose an installed Host

After ordinary `wsl -d Hacocoon` entry, run `haco doctor` or `haco doctor --json`. From Windows, `wsl -d Hacocoon --exec haco doctor --json` runs the same controller diagnostics as the ordinary WSL user. Doctor reports a stopped host without starting it. See [diagnostic scope and exit codes](design/controller-client-transport.md#host-diagnostics).

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
4. on a fresh default install, create the managed non-root `hacocoon` user with password login locked, configure it as the WSL default user, and complete the managed first-launch configuration;
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

The default managed account needs no password input. It does not reset an existing account's password on retry. For this path only, the installer replaces the known Ubuntu account/metrics OOBE command with an empty command and sets the validated default UID, preserving unrelated distribution configuration through an atomic replacement. Unknown OOBE configurations fail closed; the interactive option preserves Ubuntu's normal setup. This does not opt the user into metrics collection. See [ADR 0004](adr/0004-wsl-installer-authority.md).

## Interrupted registration and Windows restart

The installer requires a successful WSL inventory before deciding to create a distribution. A failed listing is unknown state, not an empty list. After creation, it requires the exact named distribution to appear in a second successful inventory before common Ubuntu setup can begin.

Current [WSL installation code](https://github.com/microsoft/WSL/blob/2.7.12/src/windows/common/WslClient.cpp) can print a Windows-restart request and return 0 without registering a distribution. Hacocoon therefore does not infer completion from native exit 0. Native exit 3010 is explicitly reported as restart-required and propagated through the BAT. Exit 0 without registration stops as setup-incomplete; it cannot by itself establish that a reboot is the cause. Review WSL's output and the final error before choosing the next action.

A failed creation or post-creation inventory saves `hacocoon-installation-<id>.json` next to the extracted installer. It records `wsl-registration`, restart-required/setup-incomplete, the instance, timestamp and a command retaining the validated installer options. Each attempt creates a new file without overwriting previous records. If WSL requests a restart, save your work, restart Windows, and run the printed BAT command from the same current package directory. For other failures, resolve the reported WSL failure and run that command.

The record is informational: the installer never executes or imports it, and never uses it to skip validation. Rerunning inspects actual WSL state, reuses an existing named distribution and continues the current installer. It does not unregister distributions, reboot Windows automatically, register an autorun task or execute an elevated saved command. PowerShell/BAT component tests cover the stop and continuation contract. Actual Windows OS reboot implementation/acceptance and further continuation features are outside the current requested M1 scope; disabled-feature acceptance is not claimed.

## Cached WSL image validation path

`-UseCachedWslImage` is a validation-oriented installer option for repeated Windows/WSL installation tests. It keeps the normal installer behavior unchanged unless the option is explicitly selected.

When enabled, `install-windows.ps1` uses `ubuntu.wsl` next to the installer package as the local Ubuntu 26.04 image cache. If that file is absent, the installer reads Microsoft's WSL `DistributionInfo.json`, resolves the `Ubuntu-26.04` image for the current Windows architecture, downloads it to a temporary file, verifies the published SHA256, and only then promotes it to `ubuntu.wsl`.

SHA-256 verification uses .NET directly so the BAT's Windows PowerShell 5.1 process does not depend on `Get-FileHash` module availability. Hash failures stop installation before distro creation and do not promote a partial download.

WSL creation runs as the initiating Windows user in the BAT console. WSL itself owns any prerequisite elevation; the installer does not open a separate elevated process for distro registration. Native progress/errors remain visible, and a nonzero exit code stops the installer before common Ubuntu preparation. An outdated WSL installation is not assumed to be the cause.

The download reports its destination and suppresses per-chunk PowerShell progress rendering inside this function, avoiding the PS5.1 large-file slowdown without changing the caller's progress settings.

The dedicated distribution is then created with the named-install path:

```powershell
wsl --install --from-file .\ubuntu.wsl --name Hacocoon --no-launch
```

`-UseCachedWslImage` currently supports only the `Ubuntu-26.04` base distribution and cannot be combined with `-WebDownload`. The cache file is intentionally **not** bundled into release installer packages; it is a local/CI acceleration artifact.

GitHub Actions keeps the cache trust boundary separate from untrusted pull requests. A trusted `windows-wsl-image-cache` workflow on `main` owns cache creation with `actions/cache`: on a miss it invokes the same `-UseCachedWslImage` path, so the file is downloaded through Microsoft's metadata and SHA256 validation before it is stored. The pull-request Windows installer E2E uses only `actions/cache/restore`, copies the trusted cached `ubuntu.wsl` into the extracted candidate package when available, and never writes cache state from a PR. If no trusted cache exists, the candidate installer simply performs its normal verified download for that run.

The restart/reinstall Windows E2E invokes both packaged BAT installs with `-UseCachedWslImage`, so the cached path itself remains exercised while also proving `wsl --terminate Hacocoon` restart persistence and reinstall idempotency.

PowerShell runs common preparation as WSL root and passes the selected ordinary login name as `HACO_INSTALL_USER`. The common phase resolves and validates its actual non-root UID/GID before privileged preparation and uses those exact IDs for Incus subordinate-ID mapping. It adds that user to the controller access group. No bootstrap or login sudo policy is written; `incus-admin` remains an explicit option.

## Common Ubuntu main phase

Both Windows/WSL and native Ubuntu invoke the same packaged `install.sh`.

The common main phase:

- requires Ubuntu 26.04+ and an already-running systemd;
- installs common Host dependencies;
- installs and initializes Incus unless `-SkipIncus` / its environment equivalent is selected;
- verifies the bundled architecture-specific archive checksum;
- verifies trusted GitHub/Sigstore provenance and signed release binding when provenance is enabled;
- validates the archive contains exactly the expected regular Hacocoon binaries;
- installs the Hacocoon binaries (the storage helper is removed);
- installs/restarts `haco-controller.service`;
- requires `/run/hacocoon/control.sock` to be a `root:hacocoon` mode `0660` Unix socket;
- calls product `haco setup` through the existing controller, without legacy CLI orchestration;
- proves the real trusted-host path with `/usr/local/bin/haco-host doctor` inside `haco-host`.

`install.sh` does not edit `/etc/wsl.conf`, terminate WSL, or change a user's login shell.

## Windows post phase

After the common main phase succeeds, PowerShell performs WSL-only integration.

It validates the system-owned `haco` binary, creates its `/usr/local/libexec/hacocoon-login` alias, and changes only the normal non-root WSL user's login shell. The alias talks directly to the Physical Host controller, without sudo or a `hacoq` subprocess.

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

The Windows gate builds the candidate ZIP, retains the trusted restore-only cache flow, and runs the packaged BAT with `-UseCachedWslImage`. It enters ordinary `wsl -d Hacocoon`, checks implemented product help/version, and writes a trusted-host file. It stops and reopens WSL before any installer rerun, then checks file and sudo-policy preservation across the second BAT. Root assertions are read-only: no product overrides, account/sudoers fixtures, test bridges, or mount repairs fill product gaps. The interactive account option is separate from the managed default.

The native Ubuntu gate builds the candidate `hacocoon-ubuntu-amd64.tar.gz`, extracts it, executes the packaged `install-ubuntu.sh`, and requires the controller and trusted `haco-host` round trip to succeed while confirming the native login shell was not replaced.

PR candidates are not public releases. Bundled payload checksums are verified without a product environment override; the release workflow signs and attests the exact published package. Standalone downloads still require provenance verification by default.
