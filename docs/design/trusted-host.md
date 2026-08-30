# Trusted `haco-host`

Status: partial.

## Summary

`haco-host` is Hacocoon's persistent trusted logical Host. On the local Incus backend it is an Incus system instance named `haco-host`, distinct from ordinary untrusted Environments.

The actual Linux or WSL distribution that runs the Hacocoon process, Incus daemon, loop devices, and storage mounts is the **Physical Host**. The Physical Host remains the authority for platform primitives. `haco-host` provides the normal host-like place users enter and, as later slices land, the preferred place for developer tooling and external-service operations.

```text
Physical Host / WSL
  |- Hacocoon process
  |- Incus daemon
  |- loop / Btrfs platform primitives
  `- haco-host                  TRUSTED
       |
       `- normal interactive Host UX

Managed Environments            UNTRUSTED
```

`haco-host` is part of the trusted computing base. It is not an Environment and must never be treated as an agent sandbox.

## Implemented repository slice

The current Incus implementation provides:

- `haco host ensure`, which reconciles one persistent `haco-host`;
- `haco host shell`, which ensures the instance is running and enters an interactive login shell;
- an Incus ownership marker, `user.hacocoon.role=trusted-host`, that must match before an existing `haco-host` is reused;
- rootfs placement on Hacocoon's selected managed Incus storage pool;
- the Environment name `host` reserved in the Incus backend so an Environment cannot collide with the infrastructure instance;
- WSL bootstrap support that makes the normal interactive WSL entry open `haco-host` after successful reconciliation.

The broader #275 design is not complete yet. In particular, this slice does **not** yet move Git/GitHub, OCI/containerd, cloud credentials, general external tooling, Windows mounts, or WSL interop into `haco-host`, and it does not implement the future Physical Host controller API used by #276/#277.

## Trust and authority

The Physical Host keeps Incus control authority.

`haco-host` does not receive the Incus daemon socket, `/var/lib/incus`, the Hacocoon Physical Host state directory, or an equivalent raw provider-control capability merely so a user can enter it.

```text
operator
   |
   | haco host shell
   v
Physical Host haco
   |
   | Incus control
   v
Incus daemon
   |
   v
haco-host
```

An Environment must not get direct access to `haco-host`. Future Environment-initiated privileged operations remain subject to the Hacocoon policy/capability/approval boundary rather than becoming an ambient path to the trusted Host.

## Ownership and collision handling

The literal Incus instance name `haco-host` is infrastructure-owned.

Creation writes the Hacocoon ownership marker as part of `incus init`. Reconciliation of an existing instance requires that exact marker. If an unrelated or legacy instance already occupies `haco-host`, Hacocoon fails closed instead of taking it over, starting it, deleting it, or modifying it.

The ordinary Environment name `host` would map to the same provider-local `haco-host` name, so the Incus adapter rejects that Environment name before touching Incus.

Concurrent `ensure` calls may race. If one creator wins, the loser may reuse the result only after the exact ownership marker is verified. Unexpected Incus states fail as incompatible state rather than being guessed through.

## Storage

`haco-host` uses the root storage pool selected by the normal Hacocoon Incus storage integration. On the default local backend this keeps the instance rootfs in Hacocoon's sparse-raw Btrfs-backed Incus pool rather than making `/var/lib/containerd`, repository data, or other future Host state depend on an unmanaged Physical Host filesystem location.

This placement does not by itself make all future `haco-host` data COW-shared with Seeds or Environments. Any such physical-sharing claim requires separate measurement.

## WSL default entry

After the supported Windows installer finishes successfully, the dedicated WSL distribution's normal non-root user has a dedicated login shell entry named `hacocoon-login`.

That entry is the same trusted `haco` binary invoked under a distinct executable name. On an interactive no-command launch it delegates to:

```text
sudo -n <system-owned-haco> host shell
```

The installer grants that WSL user passwordless sudo authority only for the exact `haco host ensure` and `haco host shell` commands. It does not grant `incus-admin` by default and does not expose the Incus socket to `haco-host`.

Therefore the normal UX is:

```powershell
wsl -d Hacocoon
```

```text
Physical Host login entry
    -> haco host shell
    -> haco-host
```

Explicit WSL commands remain Physical Host commands, and the root account keeps its normal shell. This preserves an emergency path such as:

```powershell
wsl -d Hacocoon -u root
```

The installer changes the normal user's login shell only after `haco host ensure` succeeds. A failed bootstrap therefore leaves a Physical Host recovery path instead of redirecting the user into a broken automatic entry loop.

When `-SkipIncus` is selected, automatic `haco-host` entry is not configured because Hacocoon cannot prove that the required backend is ready.

## Interactive warning

`haco host shell` prints a short privileged-environment warning before entering `haco-host`. Japanese locale settings receive the Japanese wording; other locales receive the English wording.

The warning is emitted only on the interactive Host-shell path. Non-interactive WSL commands are not decorated with it.

## Planned follow-up

The following remain separate follow-up work:

- make `haco-host` the default home for Git/GitHub and selected external-service tooling;
- run the Host OCI store/containerd inside `haco-host`;
- add explicit credential injection/brokering without putting reusable credentials in ordinary Environments;
- add optional WSL/Windows interop only for the trusted Host;
- implement the Physical Host controller/control-channel model required for invoking Physical-Host-authority `haco` operations from inside `haco-host` without an Incus socket;
- complete the `haco` versus `haco-host` CLI responsibility split;
- decide and implement the long-term Workspace/repository location seam without making Core assume that repositories permanently live in `haco-host`.

## Acceptance boundary

Repository tests cover ownership reconciliation, collision refusal, stopped/running state handling, create races, CLI routing, locale warning selection, and login-mode identification. CI also validates the bootstrap shell syntax.

This is not proof that a real Windows WSL installation successfully creates the instance, changes the login shell, or enters it through a Windows terminal. Real Windows + WSL 2 + systemd + Incus acceptance remains required before claiming that host-dependent path as verified.
