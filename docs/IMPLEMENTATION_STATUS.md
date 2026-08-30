# Implementation Status

Status date: 2026-08-30, after cloud deferral, the Base/OCI CLI split, Docker compatibility lifecycle integration, the OCI Seed Builder repository slices including credential-free managed-Environment harvest, the client-neutral interaction-event contract, the reusable client-adapter contract, domain-aware Standard egress enforcement, Hacocoon-managed Btrfs rootfs pool integration, default managed Btrfs `zstd:3` compression, the normal-user managed-storage privilege boundary, and the shared structured logging foundation.

This file reports **current code reality**, not desired architecture. Hacocoon is pre-1.0; implementation does not imply API stability, production support, or real-host acceptance beyond explicitly named acceptance checks.

The current milestone position is **v0.21**. Milestones are lightweight development checkpoints: v0.17 still has acceptance work, but that partial status does not block later implemented checkpoints such as v0.18-v0.21.

| Area | Current repository reality | Milestone |
|---|---|---:|
| Secure Workspace Runtime / Workspace leases | Incus-backed Environment lifecycle, canonical Workspace identity, RO/RW leases and recovery are implemented | v0.1-v0.2 |
| Client access | status, loopback forwarding and SSH preparation/revocation are implemented | v0.3 |
| Policy / Capability | fail-closed policy, approval and audit are implemented | v0.4 |
| Git / GitHub push | privileged push is brokered on the trusted Host without exporting reusable Host credentials | v0.5 |
| Agent / orchestrator integration | `haco run`, machine output and external events are implemented; orchestration remains outside Core | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` projects capability audit records into minimized stable event types with deterministic IDs, resumable cursors, bounded batches, recovery/attention flags, and public corruption errors; observation never authorizes or executes a capability | v0.6 / cross-cutting |
| Environment routing | the provider-neutral routing seam remains implemented; **cloud implementation is currently deferred** and concrete EC2/AWS/EBS code is absent from the active tree | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` exposes client-owned DTOs for exact Environment ensure/reuse, status, loopback SSH/TCP connections, revoke/delete, `/workspace` discovery, and `pkg/interaction` batches; ordinary `haco ssh` is the non-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`, per-agent binding and `haco-agent-host` foundations are implemented | v0.8-v0.10 |
| Base lifecycle | provider-neutral Base identity and `haco base list` / `haco base inspect` / `create --base` are implemented | v0.11 |
| Resource budgets | CPU, memory, PID and root-storage budgets are modeled and Incus finite limits are enforced or rejected | v0.12 |
| Managed sandbox network | managed `haco-sandbox0`, proxy-only ACL transport guard and `haco-sandbox` profile are created/verified; bridge DNS is disabled while DHCP remains; drift fails closed | v0.13 / cross-cutting |
| Domain-aware egress authorization | Core `network.egress/connect` authority, Standard HTTP/HTTPS proxy, Host-side DNS pinning, private-address rejection, CONNECT/SNI validation, trusted Incus source-IP mapping and `haco egress serve` are implemented; real supported-Incus acceptance remains host-dependent | v0.19 implemented |
| Structured logging | shared `log/slog` foundation, INFO-default text/JSON output, Environment lifecycle operation fields, sanitized DEBUG Host-command tracing, egress authorization tracing, and defense-in-depth secret redaction are implemented across maintained executables | cross-cutting infrastructure |
| Git fetch plugin | `haco plugin git fetch <environment>` uses trusted Host Git/GitHub authority including `gh auth git-credential` for HTTPS private repositories | v0.14 |
| OCI plugin boundary | containerd/nerdctl/Docker-dependent behavior lives under optional `modules/plugin/oci`; `HACO_PLUGIN_OCI=nerdctl|docker` opts in, and Core remains valid when unset | cross-cutting |
| OCI usage telemetry | `haco plugin oci seed sample` records Environment image identity snapshots; `haco plugin oci seed recommend` ranks immutable identities over the recommendation window | v0.15 |
| OCI Seed auto-selection | deterministic top 10% eligible recommendations are marked `auto_promote=true`; this selects future Seed content only | v0.15 |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` records a deletion tombstone and can explicitly extend deletion to managed Environments | v0.16 |
| OCI deletion override | tombstones prevent silent recommendation/auto-promotion of the deleted immutable identity; `haco plugin oci image reenable <reference@sha256:...>` removes only the exact immutable tombstone | v0.16 / v0.17 integration |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build` / `haco plugin oci seed current`, per-Base `haco plugin oci seed pin` / `unpin` / `pins`, conservative `haco plugin oci seed gc` / `recover`, trusted Host acquisition, credential-free exact-image harvest from explicitly marked running managed Environments, offline no-NIC build, immutable publication/current pointer, exact-parent resolution, and pre-build interrupted-builder recovery are implemented; real-host/authenticated-registry/COW acceptance remains pending | v0.17 partial |
| Docker compatibility | `haco plugin oci docker status/prepare` validates a Base-provided genuine Docker profile, verifies pinned systemd units, refuses active vendor-daemon takeover, and enables Environment-local socket activation without making Docker a Core requirement | v0.18 implemented |
| Managed Btrfs rootfs storage | local composition lazily provisions one sparse-raw Btrfs filesystem per configured Hacocoon storage pool and pins Hacocoon-owned Incus Base/Tooling/Seed/Environment rootfs paths to the corresponding `haco-<storage-id>` pool; the ordinary CLI remains non-root and delegates only typed loop/Btrfs/mount operations through a root-owned `haco-storage-helper` that revalidates managed paths, loop backing file/inode identity and mount targets; disposable GitHub-hosted normal-user helper lifecycle acceptance is automated | v0.20 first slice implemented / host-helper acceptance |
| Managed Btrfs transparent compression | managed Btrfs mounts default to `compress=zstd:3`; non-compliant managed mounts are remounted, `compress-force` is not accepted as desired state, and existing data is not automatically recompressed | v0.21 implemented |
| Optional Local OCI Registry | Registry/proxy is optional and not required for ordinary direct upstream pulls or Seed construction | unversioned optional / deferred |

## Domain-aware egress boundary

Ordinary HTTP/HTTPS egress is enforced through the Standard proxy rather than by DNS-to-IP ACL approximation. The Incus NIC remains default deny and allows only TCP to the managed bridge gateway on the Standard proxy port. The bridge keeps DHCP but disables its dnsmasq DNS listener with `raw.dnsmasq=port=0`; unmanaged DNS or ACL configuration fails closed.

The managed profile provides HTTP(S) proxy discovery to Hacocoon Environments. The proxy derives Environment identity from trusted Incus source-IP state, routes each hostname/port/protocol request through the existing Policy / Approval / Capability / audit path, resolves DNS only on the Host after authorization, pins the public answer set per connection, and validates HTTPS CONNECT against ClientHello SNI before forwarding TLS bytes. `haco egress serve` is the foreground trusted-Host launch path so the current stdio approval provider remains usable. See [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md).

## Structured logging

Maintained executables configure one shared `log/slog` root from `HACO_LOG_LEVEL` and `HACO_LOG_FORMAT`; INFO/text is the default and JSON is available without changing stdout command results. Environment create/exec/shell/delete operations carry stable `operation`, `environment_id`, duration, result/error fields through context. The trusted Host runner adds sanitized DEBUG command metadata and classifies Incus/network/storage/Git/OCI commands without automatically logging subprocess stdout or stderr. Network egress authorization adds normalized target/protocol and request correlation at DEBUG.

The shared handler redacts known password/token/API-key, authorization/cookie, credential-bearing URL, and secret-assignment patterns as defense in depth, including at DEBUG. Call sites still must omit arbitrary headers, environments, configuration objects, private keys, request bodies, and untrusted output. ERROR ownership remains at the operation/reporting boundary; lower Host/provider layers use DEBUG diagnostics rather than duplicating ERROR. See [`reference/logging.md`](reference/logging.md).

## Client adapter boundary

`pkg/clientadapter` is the reusable adapter-facing contract for VS Code-independent clients. Exported signatures use package-owned DTOs and public error sentinels rather than `internal/core` types. The adapter can ensure/reuse an Environment only when the canonical Host Workspace and requested access mode match exactly, exposes the in-guest Workspace as `/workspace`, reconciles connection metadata, and composes the public `pkg/interaction` event contract.

SSH preparation accepts public-key material only. Clients retain their private keys and IDE configuration. Returned/reconciled SSH and TCP connections are revalidated as loopback-only; incompatible provider output is rejected and newly-created invalid connections are revoked or surfaced as recovery-required when cleanup cannot be proven. The existing `haco create` + `haco ssh` + ordinary `ssh` flow is the non-VS-Code proof. See [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md).

## Client interaction boundary

`pkg/interaction` is the reusable client-facing event contract. It reads the existing trusted capability audit stream and exposes only stable, presentation-safe fields: schema/event/request identity, UTC time, event kind, Environment/capability/action labels, attention/recovery flags, a closed failure code, and the next resume cursor.

Raw capability resources, authority attributes, opaque parameters, provider output, approval tokens, credentials, and free-form audit reasons are not part of the client schema. Browser, VS Code, code-server, JetBrains, and future adapters may independently observe/deduplicate these events; reading an event has no side effect and never substitutes for the trusted Policy/Capability approval or execution boundary. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Core/plugin boundary

With `HACO_PLUGIN_OCI` unset, Hacocoon Core must not require or probe for containerd, nerdctl, Docker CLI, Docker Engine, or a local OCI Registry. Base identity remains a Core/provider-neutral concept under `haco base ...`; OCI workload tooling lives under `haco plugin oci ...`.

The project-maintained OCI plugin profile may use containerd + nerdctl, and the Docker driver may provide genuine Docker compatibility. Neither choice defines a mandatory Hacocoon Core runtime.

## OCI Seed / storage

v0.17 has repository slices for build/publish, operations hardening, and credential-free managed-Environment harvest. The implemented path is trusted Host acquisition/cache -> offline no-NIC Seed Builder -> immutable Seed revision/current pointer -> exact-parent resolution -> normal Incus/storage-driver clone. One writable `/var/lib/containerd` must never be shared across Environments.

Explicit per-Base pins are persisted as immutable OCI identities. Deletion tombstones override recommendations and existing pins until the exact immutable identity is explicitly re-enabled. `haco plugin oci seed recover` reconciles exact Hacocoon temporary builders and then performs conservative GC; `haco plugin oci seed build` invokes recovery before a new build while holding the Seed build lock. GC does not manipulate Incus-owned Btrfs internals and retains current, in-use, instance-base, or externally aliased images. Deletion state is re-checked after publication so an operator deletion racing a long build cannot silently become current.

For an exact immutable identity already present in an explicitly marked running Hacocoon-managed Environment, Seed acquisition can copy a temporary `nerdctl save` OCI archive into the trusted Host cache and then delete the guest archive. It does not copy registry credentials, credential-helper output, workspace data, arbitrary Environment files, or live containerd state. Legacy/unmarked Environments are not inspected; failed or unavailable harvest falls back to the existing trusted Host pull path.

v0.20 generalizes the local rootfs storage boundary beyond Seed-specific COW work. The local application configures a lazy Hacocoon Btrfs provider; before an Environment, Tooling Base builder, or Seed builder needs root storage, Incus resolves the managed attachment and selects the corresponding `haco-<storage-id>` pool. Base, Tooling, Seed, Environment rootfs volumes, snapshots, and clones therefore share the configured Hacocoon-managed Btrfs filesystem while Host Workspaces remain bind-mounted outside it.

Normal local composition no longer requires the whole `haco` process to run as root for managed Btrfs. Sparse file/state work remains under the invoking user, while exact privileged storage operations are translated into typed requests to the root-owned `haco-storage-helper`. The helper rejects arbitrary command execution, paths, devices and mount options; it revalidates caller-owned managed directories/images, hardlink/symlink state, loop `BACK-FILE`/`BACK-INO`, filesystem signatures before format and mount identity/postconditions. Hacocoon does not install a passwordless sudo rule. A dedicated GitHub-hosted Ubuntu 26.04 acceptance runs the Go test process as the ordinary runner user and exercises the real helper lifecycle on a disposable sparse Btrfs image.

v0.21 makes transparent compression the managed default: `compress=zstd:3` is applied on initial mount and to non-compliant already-mounted managed filesystems. `compress-force` is intentionally not the desired state, and Hacocoon does not automatically defragment/recompress old extents because doing so can reduce reflink/COW sharing. See [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md).

Local Registry is not a prerequisite and has no reserved milestone. Remaining storage acceptance includes authenticated/private-registry combinations using Host-owned credentials without leakage, physical Btrfs compression ratio and CPU-cost measurement, COW/compaction/sparse-hole behavior, broader real-host failure injection, Windows/WSL behavior, and supported-host verification beyond the automated GitHub-hosted helper lifecycle.

## Docker compatibility

v0.18 is implemented at the repository gate. This code landed while the feature was temporarily numbered v0.17 and is reclassified without rollback. `HACO_PLUGIN_OCI=docker` exposes `haco plugin oci docker status <environment>` and `prepare <environment>`. `prepare` does not install packages or mount Host sockets: it requires the selected Base/Seed to provide Docker CLI, dockerd, containerd, systemd, the docker group, and the Hacocoon-pinned socket/service units. It fails closed on unit drift or an already-active vendor Docker daemon instead of silently taking it over.

Real Incus/systemd acceptance remains host-dependent and is tracked separately from repository implementation status.

## Cloud status

v0.7 retains the provider-neutral Environment routing seam because that architecture remains useful. The former concrete EC2/AWS/EBS implementation was intentionally removed while the local/provider contracts are still moving. **Cloud implementation is currently deferred** and must not be described as active or accepted.

## Acceptance gaps

Repository tests do not substitute for real-host acceptance. Real Incus networking/resource behavior—including the proxy-only bridge ACL and dnsmasq DNS disablement—Windows/WSL + VS Code, private-registry credentials, Docker compatibility, physical managed-Btrfs compression/COW/compaction behavior, broader storage failure injection, and future cloud adapters remain environment-dependent. The dedicated GitHub-hosted storage-helper acceptance covers the ordinary-user privileged loop/Btrfs/mount lifecycle only. Partial acceptance in an earlier milestone does not prevent later minor checkpoints from advancing.
