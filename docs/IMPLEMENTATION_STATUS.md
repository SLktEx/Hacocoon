# Implementation Status

Status date: 2026-08-30, after cloud deferral, the Base/OCI CLI split, the feature-version rebaseline through v0.18, Docker compatibility lifecycle integration, the first OCI Seed Builder repository slice, explicit Seed pin/re-enable selection, the client-neutral interaction-event contract, and the reusable client-adapter contract.

This file reports **current code reality**, not desired architecture. Hacocoon is pre-1.0; implementation does not imply API stability, production support, or real-host acceptance.

The fully implemented product progression is currently contiguous through **v0.16** because v0.17 remains a partial feature gate. v0.18 Docker Compatibility has a complete repository implementation that landed before this final numbering reorder.

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
| Managed sandbox network | managed `haco-sandbox0`, ACL substrate and `haco-sandbox` profile are created/verified; drift fails closed | v0.13 |
| Git fetch plugin | `haco plugin git fetch <environment>` uses trusted Host Git/GitHub authority including `gh auth git-credential` for HTTPS private repositories | v0.14 |
| OCI plugin boundary | containerd/nerdctl/Docker-dependent behavior lives under optional `modules/plugin/oci`; `HACO_PLUGIN_OCI=nerdctl|docker` opts in, and Core remains valid when unset | cross-cutting |
| OCI usage telemetry | `haco plugin oci seed sample` records Environment image identity snapshots; `haco plugin oci seed recommend` ranks immutable identities over the recommendation window | v0.15 |
| OCI Seed auto-selection | deterministic top 10% eligible recommendations are marked `auto_promote=true`; this selects future Seed content only | v0.15 |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` records a deletion tombstone and can explicitly extend deletion to managed Environments | v0.16 |
| OCI deletion override | tombstones prevent silent recommendation/auto-promotion of the deleted immutable identity | v0.16 |
| OCI explicit Seed selection | `haco plugin oci seed pin/unpin/re-enable` persists immutable operator intent separately from telemetry; pin and auto-promotion remain distinct, re-enable requires prior deletion, and any later deletion wins again | v0.17 partial |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build/current`, explicit immutable operator selection, persisted Tooling/Seed manifests, trusted Host acquisition, offline no-NIC Seed build, immutable publication/current pointer, and exact-parent Seed resolution are implemented; real-host/COW acceptance and GC/recovery remain pending | v0.17 partial |
| Docker compatibility | `haco plugin oci docker status/prepare` validates a Base-provided genuine Docker profile, verifies pinned systemd units, refuses active vendor-daemon takeover, and enables Environment-local socket activation without making Docker a Core requirement | v0.18 implemented |
| Optional Local OCI Registry | Registry/proxy is optional and not required for ordinary direct upstream pulls or Seed construction | unversioned optional / deferred |

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

v0.17 has a first repository slice. The implemented path is trusted Host acquisition/cache -> offline no-NIC Seed Builder -> immutable Seed revision/current pointer -> exact-parent resolution -> normal Incus/storage-driver clone. Explicit immutable pin/re-enable selection is now persisted separately from usage telemetry: pin forces future Seed selection without pretending to be automatic promotion, tombstoned identities require explicit re-enable, and a later deletion suppresses an older re-enable/pin again. These selection actions grant no network or credential authority. One writable `/var/lib/containerd` must never be shared across Environments.

Local Registry is not a prerequisite and has no reserved milestone. Remaining v0.17 work includes conservative old Tooling/Seed revision GC, restart/crash recovery, authenticated/private-registry combinations, physical Btrfs COW measurement, and broader real-host acceptance.

## Docker compatibility

v0.18 is implemented at the repository gate. This code landed while the feature was temporarily numbered v0.17 and is reclassified without rollback. `HACO_PLUGIN_OCI=docker` exposes `haco plugin oci docker status <environment>` and `prepare <environment>`. `prepare` does not install packages or mount Host sockets: it requires the selected Base/Seed to provide Docker CLI, dockerd, containerd, systemd, the docker group, and the Hacocoon-pinned socket/service units. It fails closed on unit drift or an already-active vendor Docker daemon instead of silently taking it over.

Real Incus/systemd acceptance remains host-dependent and is tracked separately from repository implementation status.

## Cloud status

v0.7 retains the provider-neutral Environment routing seam because that architecture remains useful. The former concrete EC2/AWS/EBS implementation was intentionally removed while the local/provider contracts are still moving. **Cloud implementation is currently deferred** and must not be described as active or accepted.

## Acceptance gaps

Repository tests do not substitute for real-host acceptance. Real Incus networking/resource behavior, Windows/WSL + VS Code, private-registry credentials, Docker compatibility, and future cloud adapters remain environment-dependent. v0.17 additionally still needs conservative old Tooling/Seed revision GC, restart/crash recovery, authenticated/private-registry combinations, and physical Btrfs COW measurement.
