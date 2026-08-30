# Trusted `haco-host`

Status: partial.

## Summary

`haco-host` is Hacocoon's persistent trusted logical Host. On the local Incus backend it is an Incus system instance named `haco-host`, distinct from ordinary untrusted Environments.

The actual Linux or WSL distribution that runs the Physical Host controller, Incus daemon, loop devices, and storage mounts is the **Physical Host**. It remains the authority for platform primitives. `haco-host` is the normal host-like place users enter and, as later slices land, the preferred place for developer tooling and external-service operations.

```text
Physical Host / WSL
  |- haco-controller
  |- Incus daemon
  |- loop / Btrfs platform primitives
  `- haco-host                  TRUSTED
       |- client-only haco-host CLI
       `- /run/hacocoon/control.sock
            |
            `- narrow Incus Unix proxy -> Physical Host controller

Managed Environments            UNTRUSTED
```

`haco-host` is part of the trusted computing base. It is not an Environment and must never be treated as an agent sandbox.

## Implemented repository slice

The current local Incus implementation provides:

- `haco host ensure`, which reconciles one persistent `haco-host` and its client control path;
- `haco host shell`, which ensures that path is ready and enters an interactive login shell;
- an Incus ownership marker, `user.hacocoon.role=trusted-host`;
- rootfs placement on Hacocoon's selected managed Incus storage pool;
- the Environment name `host` reserved so an Environment cannot collide with the infrastructure instance;
- client-only `/usr/local/bin/haco-host` provisioning into the trusted instance;
- a trusted-host-only Incus `proxy` device that presents `/run/hacocoon/control.sock` in the instance and connects it to the Physical Host Hacocoon controller socket;
- exact validation of existing proxy configuration, failing closed on mismatches;
- guest-side `haco-host doctor` verification before `haco host ensure` succeeds;
- WSL bootstrap support that starts the Physical Host `haco-controller` systemd service, verifies readiness, reconciles `haco-host`, and only then makes normal interactive WSL entry open `haco-host`;
- real Ubuntu 26.04 + Incus + managed-Btrfs CI acceptance of the trusted-host control path.

The broader #275 design is still partial. Git/GitHub, OCI/containerd, cloud credentials, general external tooling, Windows mounts, and WSL interop have not yet been moved into `haco-host`. Physical-Host-authority `haco` commands also still need migration to the controller-client interface before the current `haco` binary can safely be provisioned inside the trusted instance.

## Trust and authority

The Physical Host keeps Incus and platform control authority.

`haco-host` does not receive the Incus daemon socket, `/var/lib/incus`, the Physical Host Hacocoon state directory, or a broad Physical Host filesystem mount. It receives only the Hacocoon-owned client endpoint required for the operations exposed by the controller API.

```text
haco-host client
   |
   | /run/hacocoon/control.sock
   v
Incus unix proxy (bind=instance)
   |
   v
Physical Host haco-controller
   |
   | Incus API/socket remains here
   v
incusd
```

Ordinary Environments do not receive the `haco-control` proxy device or the Physical Host controller socket path.

The trusted-host reconciler verifies the exact ownership marker before provisioning. An existing `haco-control` proxy is reused only when `listen`, `connect`, `bind`, `uid`, `gid`, and `mode` match the Hacocoon-managed configuration. Unexpected configuration is incompatible state rather than something to take over silently.

## Ownership and collision handling

The literal Incus instance name `haco-host` is infrastructure-owned.

Creation writes the Hacocoon ownership marker as part of `incus init`. Reconciliation of an existing instance requires that exact marker. If an unrelated or legacy instance already occupies `haco-host`, Hacocoon fails closed instead of taking it over, starting it, deleting it, or provisioning the client channel into it.

The ordinary Environment name `host` would map to the same provider-local name, so the Incus adapter rejects it before touching Incus.

Concurrent `ensure` calls may race. If one creator wins, the loser may reuse the result only after exact ownership is verified. Unexpected Incus states fail as incompatible state instead of being guessed through.

## Storage

`haco-host` uses the root storage pool selected by the normal Hacocoon Incus storage integration. On the default local backend this places the instance rootfs in Hacocoon's sparse-raw Btrfs-backed Incus pool rather than making future Host state depend on an unmanaged Physical Host filesystem location.

This placement does not by itself imply that future `haco-host` data is physically COW-shared with Seeds or Environments. Such claims require separate measurement.

## Client provisioning

`haco host ensure` resolves a compatible client-only `haco-host` binary. A test/development override may select another absolute file, but the reconciler rejects missing, non-regular, or group/world-writable candidates.

The binary is installed in the trusted instance as:

```text
/usr/local/bin/haco-host
```

The current slice intentionally does **not** install the ordinary `haco` binary inside the instance. That binary still contains direct local-composition paths. Provisioning it before its Physical-Host-authority commands are migrated to the controller-client interface could accidentally target guest-local state or a guest-local Incus installation.

The trusted `haco-host` CLI currently exposes Environment create/list/status/exec/shell/delete plus `doctor`, all through the Physical Host controller.

## WSL default entry and controller service

The supported WSL bootstrap installs the release, then configures `haco-controller` as a Physical Host systemd service. The service uses the managed Hacocoon root and owns the local controller socket under `/run/hacocoon`.

Bootstrap verifies both the socket and a real `haco-host doctor` request before it proceeds. It then runs `haco host ensure`, which provisions the trusted instance client and verifies a second doctor request from inside the instance.

Only after those checks succeed does bootstrap change the dedicated WSL distribution's normal non-root user's login shell to the `hacocoon-login` entry.

Interactive no-command launch therefore becomes:

```text
wsl -d Hacocoon
  -> hacocoon-login
  -> sudo -n <system-owned-haco> host shell
  -> verified haco-host
```

The installer grants passwordless sudo only for the exact `haco host ensure` and `haco host shell` commands. It does not grant `incus-admin` by default.

Explicit WSL commands remain Physical Host commands, and root keeps its normal shell, preserving recovery such as:

```powershell
wsl -d Hacocoon -u root
```

`-SkipIncus` leaves the Physical Host login unchanged because Hacocoon cannot guarantee the required backend/controller path in that mode.

## Interactive warning

`haco host shell` prints a short privileged-environment warning before entering `haco-host`. Japanese locale settings receive Japanese wording; other locales receive English wording. Non-interactive WSL commands are not decorated with this message.

## Planned follow-up

The following remain separate work:

- make `haco-host` the default home for Git/GitHub and selected external-service tooling;
- run the Host OCI store/containerd inside `haco-host`;
- add explicit credential injection/brokering without putting reusable credentials in ordinary Environments;
- add optional WSL/Windows interop only for the trusted Host;
- migrate Physical-Host-authority `haco` operations to the controller-client interface and then provision the appropriate `haco` client UX inside `haco-host`;
- complete the `haco` versus `haco-host` CLI responsibility split;
- decide and implement the long-term Workspace/repository location seam without making Core assume that repositories permanently live in `haco-host`;
- add PTY resize framing and generic Environment port forwarding to the controller stream layer.

## Acceptance boundary

Repository tests cover ownership reconciliation, collision refusal, stopped/running state handling, create races, client-binary validation, proxy creation/reuse/mismatch refusal, CLI routing, locale warning selection, and login-mode identification.

GitHub-hosted Ubuntu 26.04 acceptance with real Incus and managed Btrfs proves that the trusted instance can receive the client-only binary, connect through the dedicated Unix proxy, run `doctor`, and perform Environment lifecycle/exec operations through the Physical Host controller while ordinary Environments do not receive that channel.

This is still not proof of the complete Windows user journey. Real Windows terminal -> WSL 2 -> systemd -> default `haco-host` login behavior remains host-dependent acceptance.
