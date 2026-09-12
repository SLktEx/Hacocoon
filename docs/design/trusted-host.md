# Trusted `haco-host`

Implemented: Host `haco setup` waits for controller readiness through bounded read-only Ping probes before sending setup once. A failed setup response is never retried automatically. This handles the interval between systemd service activation and socket readiness without adding CLI steps.


## Notification companion

Implemented: setup also provisions same-release `/usr/local/bin/haco-notify`,
validating all required companions before provider mutation. Provisioning reuses
Host ownership, digest and root-owned executable metadata checks. Notifications
read the existing controller endpoint in controller mode; the Host does not need
the Physical Host audit file. See [interaction events](../INTERACTION_EVENTS.md).
Fresh packaged acceptance for this addition remains pending.


Status: partial.

The managed-repository WSL slice is **implemented**: registered upstream clones
and GitHub authentication live in this trusted Host. Independent Workspace
volume copies are detached before Environment use. Git-only broker requests
invoke fixed trusted Git operations; the Physical Host retains all controller,
Policy and Incus authority. See the
[workflow](../reference/managed-repository-workflow.md) and
[ADR 0008](../adr/0008-managed-repository-workspaces.md). Windows drive/exe
integration is reconciled by normal Windows installation and setup; see below.

## Windows interop

The normal Windows installer captures only Windows PATH entries already
converted by WSL and stores them in the root-owned Physical Host configuration.
Controller-backed setup projects actual mounted DrvFs drive roots, read-only
`/init` and the WSL interop socket directory into the owned `haco-host`.
No drive-letter list is compiled into the product. Linux PATH entries from the
Physical Host are excluded; the trusted Host keeps its own Linux PATH.

Fresh WSL already registers the native `WSLInterop` binfmt handler. Hacocoon
reuses it and WSL's `/init`, sets the stable init interop socket path and adds the
Windows PATH to trusted shell startup. The socket directory is mounted outside
transient `/run`; standard systemd tmpfiles restores `/run/WSL` as a symlink to
that read-only projection at every boot, preserving native absolute socket
symlinks. It does not register another handler or
create a Windows executable launcher. Healthy native binfmt registration is left
untouched; if it disappeared, setup asks WSL's own generated systemd integration
to restore it. Validation accepts only `flags: P` or `flags: PF`, with the exact
enabled `/init`, offset-zero, `4d5a` registration fields. Every `WSLInterop*`
entry must match, including after restoration; disabled, unlisted or incompatible
registrations are rejected. In a new trusted shell:

```bash
cmd.exe /c ver
powershell.exe -NoProfile -NonInteractive -Command "[Console]::Out.WriteLine('hello'); exit 23"
echo $?  # 23
```

For Windows tools sensitive to UNC current directories, first `cd` to an
available projected Windows directory. Read/write operations affect the actual
Windows filesystem and survive Environment/Host recreation; Windows ACLs still
apply. Projection devices persist in Incus and setup reconciles their identity.
The installer and ordinary setup can be rerun. Drive hotplug/removal and generic
recovery remain deferred.

Only trusted `haco-host` receives these mounts, PATH and executable authority.
Environment creation uses explicit devices without inherited profiles. It never
receives `/init`, WSL sockets, Windows drives or the trusted controller socket.
See [ADR 0009](../adr/0009-trusted-host-windows-interop.md) and the commit-bound
fresh-install/restart results in [implementation status](../IMPLEMENTATION_STATUS.md).

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

- `haco setup`, which reconciles one persistent `haco-host`;
- ordinary `wsl -d Hacocoon` entry and the retained legacy `hacoq host shell` alias;
- the ownership marker `user.hacocoon.role=trusted-host`;
- rootfs placement on Hacocoon-managed Incus storage;
- Environment name `host` reserved to avoid a provider-local collision;
- a Physical Host `haco-controller` Unix-domain endpoint;
- a dedicated `haco-control` proxy visible only in the trusted instance;
- `/usr/local/bin/haco-host` provisioning with digest/ownership verification;
- same-release `/usr/local/bin/haco` provisioning with the same source/digest/metadata checks;
- `environment.HACO_CLIENT_MODE=controller`, which prevents still-unmigrated `haco` commands from silently using guest-local composition;
- supported WSL bootstrap that verifies `haco-host doctor` before enabling default interactive entry.

The broader namespace migration, cloud credentials and general external tooling remain partial. Git/GitHub and Windows integration above are implemented. Current OCI Stores attach only to Environments and do not require a Host runtime.

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

`haco setup` provisions both release client binaries:

```text
/usr/local/bin/haco-host
/usr/local/bin/haco
```

The Physical Host source for each binary must be a regular executable, owned by the invoking effective UID, and not writable by group/other users. Hacocoon compares SHA-256 plus final `0755 root:root` metadata before deciding whether a push is necessary.

This makes repeated ensure idempotent and avoids trusting arbitrary pre-existing executables in the trusted instance.

The product `haco` binary has no guest-local composition fallback and does not invoke `hacoq`. The temporary `hacoq` remains in the Physical Host release payload for unmigrated operations; fresh trusted-host setup no longer provisions it. Existing guest copies are not a product dependency. Its controller-mode guard still refuses guest-local operations.

The mode marker is not an authorization credential. `haco-host` is already trusted, and the Physical Host controller remains the authority for policy, state, and provider operations.

## Dedicated trusted-host network

The Incus adapter owns `haco-host0` in the default resource project, marked `user.hacocoon.owner=trusted-host-network-v1`. It verifies the owner, managed bridge type, private IPv4 subnet, DHCP/DNS/NAT/routing/firewall settings and consumers before use. Unknown routing/DNS overrides, external interfaces or foreign consumers fail closed. IPv6 is disabled in this initial trusted-network contract.

The Ubuntu installer explicitly installs `dnsmasq-base` for Incus bridge DNS/DHCP, including when Incus was previously installed without recommended packages. Package installation failure stops preparation before daemon readiness or trusted-host setup; no additional `haco` option or manual DNS step is required.

Fresh trusted hosts have an explicit local NIC/root disk and no inherited profiles. Common installation checks Incus readiness and does not call minimal initialization or create a default directory pool. Existing exactly owned hosts using the known default-profile `incusbr0` NIC are gracefully stopped once, migrated to the explicit NIC and restarted; root disk, UUID and files are retained. Unknown profiles/devices fail without migration. A failed migration is resumable and never deletes the old shared bridge, profile or storage pool.

Before bootstrap/entry, the adapter checks IPv4 forwarding and reconciles Docker's `DOCKER-USER` extension point when present. Its two rules match only this bridge/subnet's outbound packets and established/related replies. Global FORWARD policy and Environment bridges are untouched. DROP without a supported extension point fails explicitly. A firewall reload or late Docker startup during an open session is not continuously reconciled; another entry rechecks the state.

The installer verifies DNS, a default IPv4 route and HTTPS inside the real trusted host before reporting success. These infrastructure checks are separate from Environment proxy/default-deny acceptance. See [ADR 0005](../adr/0005-trusted-host-network-ownership.md). Repository regressions and isolated Linux packet checks are separate from final packaged Windows acceptance.

## Storage

`haco-host` uses the root storage pool selected by the normal Hacocoon Incus storage integration. On the default local backend this keeps the instance rootfs in Hacocoon's sparse-raw Btrfs-backed Incus pool.

This does not by itself prove that all future `haco-host` data is physically COW-shared with Base images or Environments. Physical sharing remains measurement-dependent.

## WSL default entry

The common Ubuntu installer installs an Incus-specific startup guard. Across
PID namespace boots it archives old network/proxy process records before Incus
can replay a reused PID against a new worker. Same-namespace service restarts
retain records. Unknown or unsafe metadata refuses startup; no process is
signalled and no resource or Workspace is removed. See
[ADR 0013](../adr/0013-incus-pid-record-boot-identity.md) for initialization,
durability, trust boundaries and the remaining upstream scope.

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
5. `haco setup` reconciles the trusted Host, proxy, client mode, and both client binaries;
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

- extend trusted external-service tooling beyond the implemented Git/GitHub path;
- evaluate additional optional OCI runtime compatibility; current Stores attach only to Environments;
- broker credentials without putting reusable credentials in ordinary Environments;
- evaluate wider Windows application compatibility beyond the accepted native CLI cases;
- classify and migrate the remaining appropriate `haco` commands to the controller client path;
- move trusted Host-local operations into their long-term `haco-host` namespaces and remove temporary ambiguity;
- finish the `haco` versus `haco-host` CLI responsibility split;
- implement the long-term Workspace/repository location seam without making Core assume repositories permanently live in `haco-host`.

## Acceptance boundary

Repository tests cover ownership reconciliation, collision refusal, state recovery, exact controller-proxy validation, both client binaries' provisioning/idempotency, client-mode drift refusal, CLI routing, fail-closed fallback prevention, warning selection, and login-mode identification.

The maintained real Incus E2E gate checks controller-owned `haco setup`, endpoint projection, digest equality of both required clients, `haco-host doctor` and `haco-host env ...` through the Physical Host controller, restart recovery, absence of guest `hacoq` after fresh setup, raw Incus-socket non-exposure, and absence of the trusted endpoint/client-mode marker on ordinary Environments. Retained legacy aliases, Base routing and local-composition guards have component coverage. The updated gate passed on `b71f88e`; commit-bound Windows results and remaining limits are recorded in [implementation status](../IMPLEMENTATION_STATUS.md).

Windows/WSL claims are limited to the commit-bound real-host acceptance in implementation status. Other hardware and configurations remain unverified.

## Saved customization recipes

Status: **implemented explicit controller setup/replay; Windows GHA acceptance passed at bcc1baf**.

`haco setup --script <path>` saves and runs a user-selected Bash recipe after normal
Host preparation. `haco setup` replays its saved snapshot; editing the original file
has no effect until another explicit `--script` update. `haco setup --clear-script`
removes the saved recipe without executing it. No new top-level command is required.

The client reads a regular UTF-8 file of at most 1 MiB. UTF-8 BOM and Windows CRLF
are normalized. The controller stores the snapshot privately at
`/var/lib/hacocoon/host-customization/recipe.sh` (under its configured Hacocoon root).
It verifies the owned trusted Host and supplies the bytes on stdin to
`/bin/bash -se` in `/root` through a fixed transient systemd unit. The unit refuses
overlapping runs and stops its process group after at most 14 minutes, including
when the controller exits. A shorter request deadline reduces that limit. The recipe is never executed on the Physical Host and
is never copied to an Environment. Project files are not searched for hooks.

For example, inside trusted `haco-host`:

```sh
cat > ~/host-setup.sh <<'SH'
install -d -m 0755 "$HOME/.local/bin"
cat > "$HOME/.local/bin/hello-haco" <<'HELLO'
#!/bin/sh
echo "hello from haco-host"
HELLO
chmod 0755 "$HOME/.local/bin/hello-haco"
SH
haco setup --script ~/host-setup.sh
haco setup
haco setup --clear-script
```

Write replayable recipes. Setup reports failure and retains the saved recipe if a
step fails; it does not roll back earlier user commands. Script stdout/stderr are
not forwarded to controller diagnostics because they may contain credentials.
To inspect a recipe's own output, run the original script directly in the trusted
Host. Unsafe stored-file permissions or links fail closed and require inspection
of the controller-owned configuration. Explicit controller setup after Host recreation can reuse the snapshot; real
recreation acceptance and implicit recreation outside setup remain unverified. See [ADR 0019](../adr/0019-trusted-host-customization.md).

## Nested OCI runtimes

The maintained OCI setup integration enables `security.nesting=true` only after
verifying the unprivileged owned Host and its canonical ready source area.
Missing ownership, inherited profiles, paused/pending copies or ambiguous
provider results refuse setup. The setting persists; repeated setup revalidates
and reuses it. See [ADR 0032](../adr/0032-owned-host-nested-runtime.md).
Runtime binaries remain optional and actual image recovery requires separate
Docker/nerdctl acceptance.
