# Trusted `haco-host`

Status: partial.

Current CLI boundary: product `haco` implements help/version, controller-backed `doctor`, and the WSL login alias. Retained lifecycle commands described below use temporary `hacoq` during [CLI migration](../CLI_MIGRATION.md); they do not describe implemented new product commands.

Current packaged Windows acceptance and remaining gaps are recorded in [implementation status](../IMPLEMENTATION_STATUS.md). Product diagnostics use the [read-only controller contract](controller-client-transport.md#host-diagnostics).

## Summary

`haco-host` is Hacocoon's persistent trusted logical Host. On the local Incus backend it is an Incus system instance named `haco-host`, distinct from ordinary untrusted Environments.

The actual Linux or WSL distribution that runs the Hacocoon controller, Incus daemon, loop devices, and storage mounts is the **Physical Host**. The Physical Host remains the authority for platform primitives. `haco-host` is the normal host-like place users enter and the intended home for future developer/external-service tooling.

```text
Physical Host / WSL
  |- haco-controller
  |- Incus daemon
  |- loop / Btrfs platform primitives
  `- haco-host                         TRUSTED
       |- haco-host CLI
       |- guarded general haco client
       `- Hacocoon controller UDS only

Managed Environments                   UNTRUSTED
```

`haco-host` is part of the trusted computing base. It is not an Environment and must never be treated as an agent sandbox.

## Implemented repository slice

The current implementation provides:

- `hacoq host ensure`, which reconciles one persistent `haco-host`;
- `hacoq host shell`, which ensures the instance is running and enters an interactive login shell;
- the ownership marker `user.hacocoon.role=trusted-host`;
- rootfs placement on Hacocoon-managed Incus storage;
- Environment name `host` reserved to avoid a provider-local collision;
- a Physical Host `haco-controller` Unix-domain endpoint;
- a dedicated `haco-control` proxy visible only in the trusted instance;
- `/usr/local/bin/haco-host` provisioning with digest/ownership verification;
- same-release `/usr/local/bin/haco` provisioning with the same source/digest/metadata checks;
- `environment.HACO_CLIENT_MODE=controller`, which prevents still-unmigrated `haco` commands from silently using guest-local composition;
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

Normal Environments do not receive this proxy, its control-socket environment variable, or the trusted controller-client mode marker.

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

The supported WSL bootstrap runs `haco-controller` under systemd and verifies the socket is `root:hacocoon` mode `0660`. Membership in `hacocoon` grants privileged controller authority. The trusted-instance proxy remains root-only as shown below.

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
environment.HACO_CLIENT_MODE=controller
```

An existing endpoint configuration with a different target, mode, owner, bind direction, or socket path is incompatible state. Hacocoon does not silently repurpose it.

An unexpected non-empty client-mode value is also incompatible state. Hacocoon does not silently replace a different execution-context policy on the trusted instance.

The instance-side socket is intentionally outside `/run` so guest runtime tmpfs initialization does not hide the listener created by the Incus proxy device.

## Client provisioning

`hacoq host ensure` provisions both release client binaries:

```text
/usr/local/bin/haco-host
/usr/local/bin/haco
```

The Physical Host source for each binary must be a regular executable, owned by the invoking effective UID, and not writable by group/other users. Hacocoon compares SHA-256 plus final `0755 root:root` metadata before deciding whether a push is necessary.

This makes repeated ensure idempotent and avoids trusting arbitrary pre-existing executables in the trusted instance.

The product `haco` binary has no guest-local composition fallback and does not invoke `hacoq`. The separately provisioned temporary `hacoq` retains the older lifecycle/CLI implementation, guarded by `HACO_CLIENT_MODE=controller`. Its Environment operations use the controller, and unsupported guest-local operations fail closed.

The mode marker is not an authorization credential. `haco-host` is already trusted, and the Physical Host controller remains the authority for policy, state, and provider operations.

## Dedicated trusted-host network

The Incus adapter owns `haco-host0` in the default resource project, marked `user.hacocoon.owner=trusted-host-network-v1`. It verifies the owner, managed bridge type, private IPv4 subnet, DHCP/DNS/NAT/routing/firewall settings and consumers before use. Unknown routing/DNS overrides, external interfaces or foreign consumers fail closed. IPv6 is disabled in this initial trusted-network contract.

Fresh trusted hosts have an explicit local NIC/root disk and no inherited profiles. Common installation checks Incus readiness and does not call minimal initialization or create a default directory pool. Existing exactly owned hosts using the known default-profile `incusbr0` NIC are gracefully stopped once, migrated to the explicit NIC and restarted; root disk, UUID and files are retained. Unknown profiles/devices fail without migration. A failed migration is resumable and never deletes the old shared bridge, profile or storage pool.

Before bootstrap/entry, the adapter checks IPv4 forwarding and reconciles Docker's `DOCKER-USER` extension point when present. Its two rules match only this bridge/subnet's outbound packets and established/related replies. Global FORWARD policy and Environment bridges are untouched. DROP without a supported extension point fails explicitly. A firewall reload or late Docker startup during an open session is not continuously reconciled; another entry rechecks the state.

The installer verifies DNS, a default IPv4 route and HTTPS inside the real trusted host before reporting success. These infrastructure checks are separate from Environment proxy/default-deny acceptance. See [ADR 0005](../adr/0005-trusted-host-network-ownership.md). Repository regressions and isolated Linux packet checks are separate from final packaged Windows acceptance.

## Storage

`haco-host` uses the root storage pool selected by the normal Hacocoon Incus storage integration. On the default local backend this keeps the instance rootfs in Hacocoon's sparse-raw Btrfs-backed Incus pool.

This does not by itself prove that all future `haco-host` data is physically COW-shared with Seeds or Environments. Physical sharing remains measurement-dependent.

## WSL default entry

After the supported installer succeeds, the normal non-root WSL user's login shell becomes the dedicated `hacocoon-login` entry.

For an interactive no-command launch the product alias connects directly through:

```text
controlapi.Client.OpenTrustedHostShell
```

No sudo rule or `hacoq` subprocess is involved. Root-side installation preserves the ordinary user's exact UID/GID and grants controller access through the `hacocoon` group; it does not grant `incus-admin` by default. See [ADR 0004](../adr/0004-wsl-installer-authority.md).

Before changing that login shell, bootstrap now requires all of these to succeed:

1. Incus is active;
2. `haco-controller` is installed as a root-owned system binary;
3. `haco-controller.service` is restarted on the current release;
4. `/run/hacocoon/control.sock` is a `root:hacocoon` mode-`0660` Unix socket;
5. `hacoq host ensure` reconciles the trusted Host, proxy, client mode, and both client binaries;
6. `haco-host doctor` succeeds from inside the real trusted instance.

Only then does normal entry become:

```powershell
wsl -d Hacocoon
```

```text
Physical Host login entry
    -> product haco login alias -> Physical Host controller
    -> haco-host
```

Explicit WSL commands remain Physical Host commands. The root account keeps its normal shell, preserving recovery through:

```powershell
wsl -d Hacocoon -u root
```

When `-SkipIncus` is selected, controller/Host automatic entry is not configured.

## Interactive warning

`hacoq host shell` prints a short privileged-management warning before entering `haco-host`. Japanese locale settings receive Japanese wording; other locales receive English wording.

The warning is emitted only on the interactive Host-shell path, so non-interactive WSL commands are not polluted.

## Planned follow-up

Still separate work:

- make `haco-host` the normal home for Git/GitHub and selected external-service tooling;
- run the Host OCI store/containerd inside `haco-host`;
- broker credentials without putting reusable credentials in ordinary Environments;
- add optional WSL/Windows interop only to the trusted Host;
- classify and migrate the remaining appropriate `haco` commands to the controller client path;
- move trusted Host-local operations into their long-term `haco-host` namespaces and remove temporary ambiguity;
- finish the `haco` versus `haco-host` CLI responsibility split;
- implement the long-term Workspace/repository location seam without making Core assume repositories permanently live in `haco-host`.

## Acceptance boundary

Repository tests cover ownership reconciliation, collision refusal, state recovery, exact controller-proxy validation, both client binaries' provisioning/idempotency, client-mode drift refusal, CLI routing, fail-closed fallback prevention, warning selection, and login-mode identification.

Real Incus E2E covers trusted instance creation, control endpoint projection, production installation of both client binaries, exact general-client digest equality, `haco-host doctor` and `haco env ...` through the Physical Host controller, restart recovery, legacy Environment alias controller routing, unmigrated-command refusal before guest-local state creation, raw Incus-socket non-exposure, and absence of the trusted endpoint/client-mode marker on ordinary Environments.

Actual Windows terminal startup, WSL distribution restart behavior, login-shell transition, and Windows integration still require real Windows + WSL acceptance before being claimed as host-verified.
