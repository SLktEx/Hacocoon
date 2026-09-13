# Trusted `haco-host`

Implemented: Host `haco setup` waits for controller readiness through bounded read-only Ping probes before sending setup once. A failed setup response is never retried automatically. This handles the interval between systemd service activation and socket readiness without adding CLI steps.


## Notification companion

Implemented: setup also provisions same-release `/usr/local/bin/haco-notify`,
validating all required companions before provider mutation. Provisioning reuses
Host ownership, digest and root-owned executable metadata checks. Notifications
read the existing controller endpoint in controller mode; the Host does not need
the Physical Host audit file. See [interaction events](../reference/interaction-events.md).
Fresh packaged acceptance for this addition remains pending.


Status: partial.

The managed-repository WSL slice is **implemented**: registered upstream clones
and GitHub authentication live in this trusted Host. Independent Workspace
volume copies are detached before Environment use. Git-only broker requests
invoke fixed trusted Git operations; the Physical Host retains all controller,
Policy and Incus authority. See the
[workflow](../guides/git-workflow.md) and
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

The actual Linux or WSL distribution that runs the Hacocoon controller, Incus daemon, loop devices, and storage mounts is the **Physical Host**. The Physical Host remains the authority for platform primitives. `haco-host` is the normal host-like place users enter and the home for managed repository, Git and optional external-service tooling.

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

The broader namespace migration, cloud credentials and general external tooling remain partial. Git/GitHub and Windows integration above are implemented. The maintained local setup supplies Host tooling described below; Core and Environment runtime selection remain independent.

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

The [transport contract](controller-client-transport.md#trusted-haco-host-endpoint) owns the exact proxy, socket modes, ownership and execution-context configuration. Physical Host controller access is privileged. Only the exactly owned trusted Host receives the endpoint; incompatible target, owner, mode, bind direction or nonempty client-mode values fail closed. The stable instance-side socket stays outside /run to survive guest tmpfs initialization.

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

Ordinary product Host entry shows the [authority notice](#host-entry-language). The temporary `hacoq host shell` path retains its own short localized management warning. Neither is an invitation to run ordinary workloads with Host authority.

## Planned follow-up

Still separate work:

- extend trusted external-service tooling beyond the implemented Git/GitHub path;
- evaluate additional optional OCI runtime compatibility; Host-owned source areas and independent Environment Stores are supported;
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
Standard Host tooling is supplied by the maintained local integration. Docker
and Environment runtime selection remain optional; actual image recovery requires
runtime-specific acceptance.

## Standard Host tools

Ordinary `haco setup`, including the common Ubuntu setup called by the Windows/WSL
installer, installs Git, GitHub CLI, containerd, nerdctl and BuildKit before replaying
a saved user recipe. No per-user install script is required. The tools run rootfully
inside the owned, unprivileged `haco-host`; they do not run on the Physical Host.

| Component | Supported source/version |
|---|---|
| Git, GitHub CLI (`gh`) | Ubuntu 26.04+ configured signed package repositories, including universe; distro candidate on first installation, installed package reused on repeat setup |
| nerdctl | Official `nerdctl-full` 2.3.5 release, SHA-256 pinned separately for Linux amd64/arm64 |
| containerd / runc / BuildKit / CNI | Selected binaries from that same release: 2.3.3 / 1.5.1 / 0.31.2 / 1.9.1 |

The [official distribution](https://github.com/containerd/nerdctl/releases/tag/v2.3.5)
owns upstream component provenance. Setup downloads through HTTPS, verifies the
fixed digest and installs only allowlisted regular files. The verified archive
is cached in `/var/cache/hacocoon/host-tooling` for offline repeat setup. Conflicting
existing binaries/configuration, unsafe links or permissions fail instead of being
overwritten. Existing data migration and arbitrary custom runtime installations
remain unsupported; inspect the conflict rather than deleting image data.

`containerd.service` and `buildkit.service` are enabled and checked for readiness.
The default nerdctl namespace is `default`, with the `native` snapshotter and a
matching containerd transfer unpack configuration. Inside trusted `haco-host`,
after successful setup:

```bash
git --version
gh --version
nerdctl pull docker.io/library/busybox:latest
nerdctl run --rm docker.io/library/busybox:latest echo ready
# Run in a directory containing a Dockerfile:
nerdctl build -t example:local .
```

Image data and BuildKit cache remain under `/var/lib/hacocoon-oci/containerd` and
`/var/lib/hacocoon-oci/buildkit`; sockets stay under `/run` inside this Host.
Stop/start and repeat setup preserve the managed area. The existing
[independent Store copy](persistent-oci-store.md#default-environment-creation-flow)
retains its ownership and pause/copy/resume contract. Runtime binaries still need
to be supplied by the receiving Environment/Base integration. No Host socket,
registry credential or management authority is delivered with that copy. Docker
is not installed and its existing managed configuration/data is left intact.

Setup reports `host_packages`, `host_tooling` and `host_services` failures without
raw installer output. Each bounded transient service excludes overlapping installs
after controller loss. A retry reuses complete files and installs missing files;
it does not roll back package changes, reset OCI data or restart healthy services.
See [ADR 0063](../adr/0063-standard-trusted-host-tooling.md). Repository tests and
the dedicated real-Incus fixture are distinct from released Windows installer
acceptance and private-registry credential acceptance.

On a dedicated root Linux/WSL Incus/Btrfs test host, run the maintained fixture:

```bash
HACO_E2E_HOST_TOOLING=1 go test -count=1 -run '^TestRealIncusHostToolingE2E$' \
  -v -timeout 18m ./modules/runtime/incus
```

The fixture creates its own project and pool, and cleans them after a pass.
It uses normal Host setup to verify/configure `haco-host0`, which remains managed
infrastructure. An existing different Host consuming that network is refused.
Failure retains its printed ownership identities for inspection. See
[acceptance evidence](../status/acceptance-evidence.md#installation) for results.


## Host entry language

Implemented: the trusted Host entry notice follows the Physical Host login process's first nonempty `LC_ALL`, `LC_MESSAGES`, then `LANG`. Japanese locales select Japanese; other locales retain English. The notice still identifies Host authority and directs ordinary development into an Environment. Interactive stderr uses yellow unless `NO_COLOR` is nonempty; redirected output stays plain.

A fresh Windows installation maps Japanese Windows UI language to `ja_JP.UTF-8` through Ubuntu's locale tools before login-user setup. Existing distributions keep their locale, and other Windows languages keep Ubuntu defaults. A locale setup failure stops installation. This changes presentation only, not Host/Env authority, controller readiness, or credential forwarding. Fresh Japanese-Windows installation acceptance remains unverified.

## Setup progress and failure diagnostics

Status: **implemented**. `haco setup` observes the existing owned-resource
reconciler through the management controller. Stderr shows running/succeeded/
failed stages; stdout retains the final command result. There is no percentage
or success inferred from process dispatch. Stages cover client validation,
project/storage, owned Host inspection/creation, network, controller endpoint,
start, WSL interop, client mode/provisioning, Host storage, notifications and
customization. Repeated stages mean actual repeated reconciliation checks.
Optional stages are absent when not configured, not reported as completed.

The controller records fixed stage/state/reason, duration and a generated
`request_id` through the shared structured logger. The CLI validates this bounded
vocabulary again. Arbitrary provider errors, helper output, credentials and
recipe text are not diagnostic fields. WSL helper exit 42 specifically means
`native_binfmt_incompatible`; unknown failures remain `failed`, rather than a
guessed cause. Other reasons include timeout, canceled, incompatible_state,
recovery_required, unavailable, denied, busy, not_found and unsupported.

Use `haco doctor` to inspect current readiness. On the WSL/Linux **Physical Host**,
an administrator can read `journalctl -u haco-controller.service --since
'30 minutes ago' --no-pager` and locate the printed request ID. Journal retention
and rotation remain systemd-journald responsibilities. Existing
`HACO_LOG_LEVEL=debug` / `HACO_LOG_FORMAT=json` configure diagnostics; client
settings do not enable controller DEBUG remotely. DEBUG retains redaction.

There is no approval interaction in Host setup. A busy result means another
setup owns the operation, not that approval is pending. Capability approvals
remain separate under `haco approve`. Ctrl+C stops observation; as with the
existing lifecycle RPC, the bounded controller setup may continue after a lost
client. Exclusion is held until the actual service returns. A broken stream,
missing final acknowledgement or incompatible older controller cannot cause a
second mutating request or a successful completion display.

Completed stage lines describe that attempt, not a fresh resource inventory.
Failure retains potentially created resources; setup never promises rollback.
Inspect the request and current doctor result before choosing an explicit retry.
Saved customization can have external side effects and must not be blindly
replayed. This observation change adds no cleanup authority and changes no
ownership, lease, network or authorization invariants.
