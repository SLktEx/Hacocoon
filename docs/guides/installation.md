# Installation and Host entry

[日本語](installation.ja.md) | English

Use this guide for installer choices and interruption handling. For the complete
first development session, follow [getting started](getting-started.md).
Supported local baseline: Ubuntu 26.04+ with systemd; Windows uses a dedicated
Ubuntu 26.04 WSL 2 distribution. [Acceptance limits](../status/acceptance-evidence.md#installation)
are separate from repository implementation.

Incus must be on the **7.0 LTS** series (`>= 7.0.1`, `< 7.1`). The complete
installer selects the latest 7.0.x package from the verified Zabbly LTS source;
normal APT updates can advance patches while excluding 7.1+ feature releases.
`haco doctor` reports unsupported versions. Historical 6.0.5 installations are
outside this baseline; preserve existing data before upgrading, and never
downgrade a newer series in place. See the [package contract](../design/installer.md#incus-package-baseline).

## Choose the complete package

Download the matching `hacocoon-windows-amd64.zip` / `hacocoon-windows-arm64.zip`
or `hacocoon-ubuntu-amd64.tar.gz` / `hacocoon-ubuntu-arm64.tar.gz` from
[Releases](https://github.com/SLktEx/Hacocoon/releases). Extract the entire package.
Raw `haco_linux_*.tar.gz` assets are standalone binaries, not the normal installer.

Each package contains one matching Linux archive, scripts, checksums and VERSION.
The installer consumes that archive; network access is still needed for Ubuntu
packages and configured provenance verification. A published release can lag main;
check its identity with `haco version --json`. Do not assume a current main feature
is in an older release.

## Windows

Run in PowerShell in the extracted directory, with current WSL installed:

```powershell
.\install-windows.bat
wsl -d Hacocoon
```

The default one-invocation flow creates/reuses only the dedicated distribution,
requires WSL 2, configures systemd and creates a locked-password non-root
`hacocoon` account when fresh. Existing non-root default users/passwords are retained.
It does not change global WSL defaults, grant blanket sudo or unregister other distributions.

If Linux-first setup already created the `hacocoon` access group, managed-user preparation reuses its validated non-root GID. An absent group uses normal private-group creation; failed lookup, malformed records and GID zero stop account creation. Existing accounts/passwords stay unchanged.

Fresh Japanese Windows installations set `ja_JP.UTF-8` before login-user setup; existing distributions retain their locale. The Host entry notice follows the message locale. Fresh Japanese-Windows installation acceptance remains pending; see [Host entry language](../design/trusted-host.md#host-entry-language).

For Ubuntu's interactive account/OOBE flow, select this alternative installer invocation:

```powershell
.\install-windows.bat -InteractiveUserSetup
```

Complete account setup and exit its shell; the same installer continues.
The default managed flow clears only Ubuntu's known account/metrics OOBE command
without opting into metrics. Unknown OOBE config is refused. See [ADR 0004](../adr/0004-wsl-installer-authority.md).

Ordinary `wsl -d Hacocoon` enters trusted `haco-host`. Explicit/noninteractive WSL
commands remain on the Physical Host. Windows executables and drives are projected
only into trusted Host, never Environments. Root recovery entry remains explicit:

```powershell
wsl -d Hacocoon -u root
```

Use it for diagnosis with existing administrative authority, not ordinary workload
execution or manual ownership deletion. [Trusted Host](../design/trusted-host.md)
defines the management boundary.

## Native Ubuntu

From the extracted package directory:

```bash
./install-ubuntu.sh
```

The entry rejects WSL, checks Ubuntu/systemd and uses sudo when required.
It leaves the user's login shell unchanged. Enter trusted Host through the
temporary [Host-entry migration command](../reference/cli-migration.md#host-entry).
Normal development after entry uses product `haco`.

## Completion and diagnosis

The shared installer installs dependencies, Incus, binaries and the Physical Host
controller, validates its `root:hacocoon` mode-0660 socket, calls `haco setup`,
and verifies the actual trusted-Host controller/DNS/route/HTTPS path and doctor.
It does not infer network readiness from storage or initialize an unused default
Incus pool. Owned Btrfs storage/network reconciliation belongs to the adapter.

After normal Host entry:

```bash
haco doctor
haco doctor --json
```

Doctor reports state without repairs or starting a stopped Host. Failed/skipped/
unavailable diagnostics return nonzero; a pending mount policy is not ready.
Follow the reported next action rather than resetting data to make the check pass.

## Interrupted installation

A failed WSL listing is unknown state, not “distribution absent.” Registration must
appear in a successful second inventory. Exit 3010 reports restart-required.
Exit zero without registration reports incomplete setup; it alone does not prove a reboot cause.

A failed registration writes a new advisory `hacocoon-installation-<id>.json` next to
the package, retaining stage/options and a retry command. The installer never executes
that file or uses it to bypass checks. If WSL requests a Windows restart, save work,
restart Windows and rerun the printed command from the same current package.
Otherwise address the reported failure first. There is no automatic OS reboot or autorun.

Account/readiness probes have bounded read-only retries; mutations are not retried
through those probes. Failure never proves an existing account is missing.
Current installer reruns reconcile exactly owned resources and retain data.
Compatibility with arbitrary pre-1.0 old installers is not promised.

## Optional cached image for validation

`-UseCachedWslImage` uses `ubuntu.wsl` beside the installer for repeated tests.
On a miss it resolves Microsoft's Ubuntu-26.04 architecture entry and verifies its
SHA-256 before promotion. It cannot combine with `-WebDownload`; the cache is not
a release asset. Registration runs as the initiating Windows user; WSL owns any
prerequisite elevation. Errors remain visible in the BAT console.

Trusted main CI creates the validated cache; PR CI restores it only. A missing cache
uses the same verified download. See [CI trust boundary](../../.github/security/CI_TRUST_BOUNDARY.md).

## Reclamation helper and implementation

Normal managed Windows installation permanently installs and enrolls the verified
`haco-wsl.exe`, exact WSL GUID, installation ID, Windows owner and VHDX identity.
No added PATH step or retained extracted ZIP is needed. Identical enrollment is
reusable; conflicts are retained and refused. Public [reclamation](../design/storage-reclamation.md)
has separate dispatch, completion and review semantics.

[Installer architecture](../design/installer.md) owns phase/package details.
[Release security](../security/release-security.md) owns provenance.
Package E2E starts at BAT/native wrapper and proves user-visible readiness;
a locally built candidate is not a published/attested release.
