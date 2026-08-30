# Trusted `haco-host`

Status: partial.

## Summary

`haco-host` is Hacocoon's persistent trusted logical Host. On the local Incus backend it is an Incus system instance named `haco-host`, distinct from ordinary untrusted Environments.

The actual Linux or WSL distribution that runs the Hacocoon controller, Incus daemon, loop devices, and storage mounts is the **Physical Host**. The Physical Host remains the authority for platform primitives. `haco-host` is the normal host-like place users enter and the intended home for future developer/external-service tooling.

```text
Physical Host / WSL
  |- haco-controller
  |- Incus daemon
  |- loop / Btrfs platform primitives
  `- haco-host                         TRUSTED
       |- client-only haco-host CLI
       `- Hacocoon controller UDS only

Managed Environments                   UNTRUSTED
```

`haco-host` is part of the trusted computing base. It is not an Environment and must never be treated as an agent sandbox.

## Implemented repository slice

The current implementation provides:

- `haco host ensure`, which reconciles one persistent `haco-host`;
- `haco host shell`, which ensures the instance is running and enters an interactive login shell;
- the ownership marker `user.hacocoon.role=trusted-host`;
- rootfs placement on Hacocoon-managed Incus storage;
- Environment name `host` reserved to avoid a provider-local collision;
- a Physical Host `haco-controller` Unix-domain endpoint;
- a dedicated `haco-control` proxy visible only in the trusted instance;
- client-only `/usr/local/bin/haco-host` provisioning with digest/ownership verification;
- supported WSL bootstrap that verifies `haco-host doctor` before enabling default interactive entry.

The broader trusted-Host design is still partial: Git/GitHub, OCI/containerd, cloud credentials, general external tooling, Windows mounts, and WSL interop have not all moved into `haco-host`, and the full `haco` versus `haco-host` responsibility migration is not complete.

## Trust and authority

The Physical Host keeps Incus control authority and authoritative Hacocoon state.

`haco-host` does **not** receive:

- `/var/lib/incus/unix.socket`;
- `/var/lib/incus/unix.socket.user`;
- `/var/lib/incus`;
- the Physical Host Hacocoon state directory;
- a mounted raw provider-control socket.

Instead it receives one narrow controller path:

```text
haco-host process
  |
  | unix:/var/lib/hacocoon-control.sock
  v
Incus proxy device: haco-control
  |
  | unix:/run/hacocoon/control.sock
  v
Physical Host haco-controller
  |
  v
policy / state / provider authority
```

Normal Environments do not receive this proxy or its environment variable.

Environment-initiated privileged work must continue through the Hacocoon policy/capability/approval boundary rather than becoming an ambient path to the trusted Host.

## Ownership and collision handling

The literal Incus instance name `haco-host` is infrastructure-owned.

Creation writes the Hacocoon ownership marker as part of `incus init`. Reconciliation of an existing instance requires that exact marker. If an unrelated or legacy instance already occupies `haco-host`, Hacocoon fails closed instead of taking it over, starting it, deleting it, or changing its devices.

The ordinary Environment name `host` would map to the same provider-local name, so it is rejected before Incus mutation.

Concurrent create/device reconciliation races may be accepted only after the final owned state exactly matches the expected Hacocoon configuration.

## Controller endpoint

The Physical Host controller uses:

```text
/run/hacocoon/control.sock
```

The supported WSL bootstrap runs `haco-controller` under systemd and verifies the socket is `root:root` mode `0600`.

The trusted instance receives exactly this proxy shape:

```text
device: haco-control
type=proxy
bind=instance
listen=unix:/var/lib/hacocoon-control.sock
connect=unix:/run/hacocoon/control.sock
mode=0600
uid=0
gid=0
```

and:

```text
environment.HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock
```

An existing endpoint configuration with a different target, mode, owner, bind direction, or socket path is incompatible state. Hacocoon does not silently repurpose it.

The instance-side socket is intentionally outside `/run` so guest runtime tmpfs initialization does not hide the listener created by the Incus proxy device.

## Client provisioning

`haco host ensure` also provisions the release's client-only `haco-host` binary to:

```text
/usr/local/bin/haco-host
```

The Physical Host source must be a regular executable, owned by the invoking effective UID, and not writable by group/other users. Hacocoon compares SHA-256 plus final `0755 root:root` metadata before deciding whether a push is necessary.

This makes repeated ensure idempotent and avoids trusting an arbitrary pre-existing executable in the trusted instance.

## Storage

`haco-host` uses the root storage pool selected by the normal Hacocoon Incus storage integration. On the default local backend this keeps the instance rootfs in Hacocoon's sparse-raw Btrfs-backed Incus pool.

This does not by itself prove that all future `haco-host` data is physically COW-shared with Seeds or Environments. Physical sharing remains measurement-dependent.

## WSL default entry

After the supported installer succeeds, the normal non-root WSL user's login shell becomes the dedicated `hacocoon-login` entry.

For an interactive no-command launch it delegates to:

```text
sudo -n <system-owned-haco> host shell
```

The narrow sudo rule authorizes only exact `haco host ensure` and `haco host shell`; it does not grant `incus-admin` by default.

Before changing that login shell, bootstrap now requires all of these to succeed:

1. Incus is active;
2. `haco-controller` is installed as a root-owned system binary;
3. `haco-controller.service` is restarted on the current release;
4. `/run/hacocoon/control.sock` is a root-owned mode-`0600` Unix socket;
5. `haco host ensure` reconciles the trusted Host, proxy, and client binary;
6. `haco-host doctor` succeeds from inside the real trusted instance.

Only then does normal entry become:

```powershell
wsl -d Hacocoon
```

```text
Physical Host login entry
    -> haco host shell
    -> haco-host
```

Explicit WSL commands remain Physical Host commands. The root account keeps its normal shell, preserving recovery through:

```powershell
wsl -d Hacocoon -u root
```

When `-SkipIncus` is selected, controller/Host automatic entry is not configured.

## Interactive warning

`haco host shell` prints a short privileged-management warning before entering `haco-host`. Japanese locale settings receive Japanese wording; other locales receive English wording.

The warning is emitted only on the interactive Host-shell path, so non-interactive WSL commands are not polluted.

## Planned follow-up

Still separate work:

- make `haco-host` the normal home for Git/GitHub and selected external-service tooling;
- run the Host OCI store/containerd inside `haco-host`;
- broker credentials without putting reusable credentials in ordinary Environments;
- add optional WSL/Windows interop only to the trusted Host;
- migrate appropriate Physical-Host-authority `haco` commands to the controller client path;
- finish the `haco` versus `haco-host` CLI responsibility split;
- implement the long-term Workspace/repository location seam without making Core assume repositories permanently live in `haco-host`.

## Acceptance boundary

Repository tests cover ownership reconciliation, collision refusal, state recovery, exact controller-proxy validation, client provisioning/idempotency, CLI routing, warning selection, and login-mode identification.

Real Incus E2E covers trusted instance creation, control endpoint projection, client installation, `haco-host doctor` through the Physical Host controller, restart recovery, raw Incus-socket non-exposure, and absence of the trusted endpoint on ordinary Environments.

Actual Windows terminal startup, WSL distribution restart behavior, login-shell transition, and Windows integration still require real Windows + WSL acceptance before being claimed as host-verified.
