# Windows / WSL installation

Hacocoon uses a **dedicated WSL 2 distribution with systemd** on Windows instead of reusing a normal Ubuntu/Debian development distribution.

The dedicated distribution is the **Physical Host**. After a normal installation, its default interactive entry immediately enters the persistent trusted `haco-host` Incus instance so users normally do not need to work in the Physical Host shell.

```text
Windows desktop
  |
  +-- normal user WSL distributions      <- untouched
  |
  +-- WSL 2 distribution: Hacocoon       <- Physical Host / substrate
        |- systemd (PID 1)
        |- Hacocoon + Incus
        |- loop / Btrfs storage primitives
        |
        `- Incus: haco-host               <- TRUSTED default interactive Host
             |
             +-- future Git / OCI / external tooling
             `-- operator shell

        Incus: managed Environments       <- UNTRUSTED agent workloads

Windows VS Code / external orchestrators
  -> stable Hacocoon client surface
  -> target Environment
```

The current `haco-host` implementation slice is documented in [`design/trusted-host.md`](design/trusted-host.md).

## Normal installer

GitHub Releases publish `install-windows.ps1` as a standalone installer. You do **not** need to clone the Hacocoon repository first.

Run it from an elevated PowerShell:

```powershell
.\install-windows.ps1
```

The default dedicated WSL distribution name is `Hacocoon`; the default base distribution is `Ubuntu-26.04`.

On a fresh PC, Windows may require a reboot or the new Linux distribution may require its normal first-run user creation. Before Hacocoon bootstrap has completed, launching:

```powershell
wsl -d Hacocoon
```

still enters the base distribution normally so that initial Linux user setup can finish. Re-run `install-windows.ps1` after that setup.

Once bootstrap succeeds, the same command becomes the normal Hacocoon entry point:

```powershell
wsl -d Hacocoon
```

and enters `haco-host`.

## WSL 2 is required and enforced

Modern `wsl --install` normally creates WSL 2 distributions, but Hacocoon does not rely only on that default.

The installer checks the dedicated instance with `wsl --list --verbose`. If **that Hacocoon-owned instance only** is WSL 1, it runs:

```powershell
wsl --set-version Hacocoon 2
```

It does not run `wsl --set-default-version`, does not convert unrelated distributions, and does not change the user's global WSL defaults.

## systemd is required and enforced

The supported local Incus path requires **systemd as PID 1** inside the dedicated Physical Host distribution.

The Linux bootstrap installs the required systemd packages and preserves unrelated `/etc/wsl.conf` settings while ensuring:

```ini
[boot]
systemd=true
```

If PID 1 is not yet systemd, the Windows installer terminates only the Hacocoon distribution with:

```powershell
wsl --terminate Hacocoon
```

and retries bootstrap after restart. Hacocoon never uses `wsl --shutdown` as part of this path because that would affect unrelated WSL distributions.

An old WSL version without the required systemd support fails with a clear request to run `wsl --update`; Hacocoon does not silently update WSL itself.

## Managed storage filesystem

The supported WSL path uses Hacocoon-managed Btrfs storage. The default local backend creates a sparse raw filesystem and mounts it with `compress=zstd:3`.

`compress-force` is intentionally not used. Hacocoon also does not automatically defragment/recompress existing data because doing so can break reflink/COW sharing.

The bootstrap installs `btrfs-progs` because `haco host ensure` may be the first operation that causes the managed local rootfs pool to be created.

## Trusted `haco-host` bootstrap

After Incus and the Hacocoon release are ready, the bootstrap runs:

```text
haco host ensure
```

with Physical Host authority.

That operation:

- reconciles exactly one Incus instance named `haco-host`;
- stores its rootfs on the Hacocoon-selected managed Incus storage pool;
- creates it with `user.hacocoon.role=trusted-host`;
- refuses to take over an existing `haco-host` without that exact ownership marker;
- starts a stopped owned instance;
- fails closed on unexpected state.

The ordinary Environment name `host` is reserved by the Incus adapter because it would otherwise map to the same provider-local instance name.

The bootstrap changes the normal WSL user's login shell **only after** this reconciliation succeeds. A failed trusted-host bootstrap therefore leaves the normal Physical Host shell available for repair rather than trapping the user in a broken automatic login path.

## Why `wsl -d Hacocoon` enters `haco-host`

The installer creates a root-owned `/usr/local/libexec/hacocoon-login` entry that points at the installed `haco` binary under a dedicated executable name, then makes that the normal non-root WSL user's login shell.

The `haco` binary detects this invocation mode. For an interactive no-command WSL launch it delegates to:

```text
sudo -n <system-owned-haco> host shell
```

`haco host shell` reconciles the trusted Host, prints the privileged-environment warning, and enters `/bin/bash -l` inside `haco-host`.

Explicit shell arguments or non-interactive login-shell use stay on `/bin/bash` on the Physical Host. More importantly, WSL explicit command execution does not require the default login shell, so automation remains able to target the substrate directly.

## Privilege boundary for automatic entry

The installer does **not** grant `incus-admin` merely to make automatic entry work.

Instead, the normal WSL user receives passwordless sudo permission for only these exact system-owned commands:

```text
haco host ensure
haco host shell
```

Before installing that sudoers rule, bootstrap requires the `haco` executable to be in `/usr/local/bin/haco` or `/usr/bin/haco`, owned by root, and not group/world writable. The generated sudoers file is validated with `visudo` before installation.

The Incus daemon socket and `/var/lib/incus` are not mounted into `haco-host`.

`-GrantIncusAdmin` remains an explicit separate option for operators who intentionally want the dedicated Physical Host user to have raw Incus administrative authority. `incus-admin` is effectively root-equivalent local authority and remains opt-in.

## Physical Host escape hatch

Normal interactive use:

```powershell
wsl -d Hacocoon
```

enters `haco-host`.

The Physical Host remains explicitly reachable for bootstrap, repair, and host-only commands. The root account's shell is never replaced by the Hacocoon login entry, so the primary recovery path is:

```powershell
wsl -d Hacocoon -u root
```

Explicit commands can also be run against the Physical Host instead of the default interactive Host entry. For example:

```powershell
wsl -d Hacocoon -- haco host ensure
```

Commands that need Physical Host root authority must still be invoked with the appropriate privilege, for example through the root escape hatch or an explicitly authorized sudo rule.

If automatic entry fails, the diagnostic names the Physical Host recovery command instead of silently opening a different shell and pretending bootstrap succeeded.

## `-SkipIncus`

`-SkipIncus` continues to support deployments where Incus is managed separately, but Hacocoon cannot prove that the trusted Host backend is ready in that mode.

Therefore `-SkipIncus` leaves the default Physical Host login shell unchanged and does not configure automatic `haco-host` entry. Once an external Incus setup is ready, the trusted-host entry can be configured by a future explicit reconciliation path.

## Workspace and repository location

The default interactive entry and Workspace location are separate design concerns.

The long-term direction is to prefer Hacocoon-managed storage associated with `haco-host` for the normal WSL repository/workspace experience while keeping the location abstract enough that a future Physical Host or external Workspace provider remains possible. That relocation is part of the broader trusted-host/workspace work and is **not** completed merely by changing the WSL login shell.

Until the workspace-location migration lands, do not infer that an arbitrary path inside the `haco-host` rootfs is automatically mountable into a sibling Environment. Existing Physical Host Workspace paths can still be operated non-interactively from Windows, for example:

```powershell
wsl -d Hacocoon -- sh -lc 'cd ~/src/my-repo && haco-vscode open .'
```

VS Code and external orchestration should continue to target Hacocoon's client/control surface and the requested Environment rather than using `haco-host` as an SSH jump host.

## Installer responsibilities

The standalone installer/bootstrap path now performs the following relevant steps:

1. validate the instance name, base distribution, and Hacocoon version;
2. create or reuse only the named Hacocoon WSL distribution;
3. enforce WSL 2 for that distribution only;
4. install systemd support and ensure `systemd=true`;
5. restart only that distribution when required;
6. install Incus unless explicitly skipped;
7. install Btrfs userspace tools and the Hacocoon release;
8. reconcile the trusted `haco-host`;
9. install the narrow automatic-entry sudo rule;
10. set the normal non-root user's login shell to `hacocoon-login`.

The installer does not change the global WSL default distribution, `.wslconfig`, unrelated distributions, or the root user's login shell.

## Checkout/developer bootstrap

A repository checkout still provides:

```powershell
.\scripts\bootstrap-windows.ps1
```

It uses the checkout's `bootstrap-wsl.sh` / `install.sh` but follows the same WSL 2, systemd, Incus, and trusted-host entry contract as the standalone installer.

## Acceptance boundary

Repository CI verifies Go tests, shell/PowerShell syntax, WSL 2/systemd policy constraints, installer/release integrity contracts, and trusted-host reconciliation behavior that can be exercised without a Windows kernel.

Real-host acceptance still requires an actual Windows machine with WSL 2 + systemd + Incus to prove:

- first-run Linux user setup;
- WSL restart behavior;
- real managed Btrfs creation;
- real `haco-host` image acquisition/start;
- login-shell transition from `wsl -d Hacocoon` into `haco-host`;
- Physical Host root recovery;
- Windows VS Code / orchestration integration after the default-entry change.

Do not treat repository CI alone as proof of those Windows-host-dependent behaviors.
