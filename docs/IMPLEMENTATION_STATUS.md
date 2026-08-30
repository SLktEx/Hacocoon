# Implementation Status

Status date: 2026-08-30, after cloud deferral, the Base/OCI CLI split, the feature-version rebaseline through v0.18, Docker compatibility lifecycle integration, and the client-neutral interaction-event contract.

This file reports **current code reality**, not desired architecture. Hacocoon is pre-1.0; implementation does not imply API stability, production support, or real-host acceptance.

The fully implemented product progression is currently contiguous through **v0.17**. v0.18 is planned.

| Area | Current repository reality | Milestone |
|---|---|---:|
| Secure Workspace Runtime / Workspace leases | Incus-backed Environment lifecycle, canonical Workspace identity, RO/RW leases and recovery are implemented | v0.1-v0.2 |
| Client access | status, loopback forwarding and SSH preparation/revocation are implemented | v0.3 |
| Policy / Capability | fail-closed policy, approval and audit are implemented | v0.4 |
| Git / GitHub push | privileged push is brokered on the trusted Host without exporting reusable Host credentials | v0.5 |
| Agent / orchestrator integration | `haco run`, machine output and external events are implemented; orchestration remains outside Core | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` projects capability audit records into minimized stable event types with deterministic IDs, resumable cursors, bounded batches, recovery/attention flags, and public corruption errors; observation never authorizes or executes a capability | v0.6 / cross-cutting |
| Environment routing | the provider-neutral routing seam remains implemented; **cloud implementation is currently deferred** and concrete EC2/AWS/EBS code is absent from the active tree | v0.7 |
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
| Docker compatibility | `haco plugin oci docker status/prepare` validates a Base-provided genuine Docker profile, verifies pinned systemd units, refuses active vendor-daemon takeover, and enables Environment-local socket activation without making Docker a Core requirement | v0.17 |
| OCI Seed Builder / Btrfs COW | trusted Host acquisition/cache, offline builder, immutable Seed publish/current pointer and physical COW validation remain planned | v0.18 |
| Optional Local OCI Registry | Registry/proxy is optional and not required for ordinary direct upstream pulls or Seed construction | unversioned optional / deferred |

## Client interaction boundary

`pkg/interaction` is the reusable client-facing event contract. It reads the existing trusted capability audit stream and exposes only stable, presentation-safe fields: schema/event/request identity, UTC time, event kind, Environment/capability/action labels, attention/recovery flags, a closed failure code, and the next resume cursor.

Raw capability resources, authority attributes, opaque parameters, provider output, approval tokens, credentials, and free-form audit reasons are not part of the client schema. Browser, VS Code, code-server, JetBrains, and future adapters may independently observe/deduplicate these events; reading an event has no side effect and never substitutes for the trusted Policy/Capability approval or execution boundary. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Core/plugin boundary

With `HACO_PLUGIN_OCI` unset, Hacocoon Core must not require or probe for containerd, nerdctl, Docker CLI, Docker Engine, or a local OCI Registry. Base identity remains a Core/provider-neutral concept under `haco base ...`; OCI workload tooling lives under `haco plugin oci ...`.

The project-maintained OCI plugin profile may use containerd + nerdctl, and the Docker driver may provide genuine Docker compatibility. Neither choice defines a mandatory Hacocoon Core runtime.

## Docker compatibility

v0.17 is implemented at the repository gate. `HACO_PLUGIN_OCI=docker` exposes `haco plugin oci docker status <environment>` and `prepare <environment>`. `prepare` does not install packages or mount Host sockets: it requires the selected Base/Seed to provide Docker CLI, dockerd, containerd, systemd, the docker group, and the Hacocoon-pinned socket/service units. It fails closed on unit drift or an already-active vendor Docker daemon instead of silently taking it over.

Real Incus/systemd acceptance remains host-dependent and is tracked separately from repository implementation status.

## OCI storage direction

Physical Seed publication/COW belongs to v0.18. The intended path is trusted Host acquisition/cache -> offline Seed Builder -> immutable Seed revision -> normal Incus/storage-driver clone. One writable `/var/lib/containerd` must never be shared across Environments.

Local Registry is not a prerequisite and has no reserved milestone. See [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md).

## Cloud status

v0.7 retains the provider-neutral Environment routing seam because that architecture remains useful. The former concrete EC2/AWS/EBS implementation was intentionally removed while the local/provider contracts are still moving. **Cloud implementation is currently deferred** and must not be described as active or accepted.

## Acceptance gaps

Repository tests do not substitute for real-host acceptance. Real Incus networking/resource behavior, Windows/WSL + VS Code, private-registry credentials, Docker compatibility, and future cloud adapters remain environment-dependent. v0.18 is planned only.
