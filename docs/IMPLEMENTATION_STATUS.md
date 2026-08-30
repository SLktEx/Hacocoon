# Implementation Status

Status date: 2026-08-30, after the feature-version rebaseline through v0.19, the Base/OCI CLI namespace split, the optional-OCI-plugin boundary refactor, and the temporary removal of the concrete EC2/AWS/EBS implementation slice.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Versioned specifications are design/acceptance references; their existence does not imply full implementation.

Hacocoon is still **pre-1.0**. An implemented area does not mean its CLI/API/state/config surface is frozen, production support is guaranteed, or every real-provider/client acceptance test has passed.

The fully implemented product progression is currently contiguous through **v0.16**. v0.17 has a Docker compatibility foundation plus the OCI plugin boundary/driver composition, but complete Engine/Base integration is still pending. v0.18-v0.19 are planned.

| Area | Current repository reality | Milestone | Validation status |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace`, `haco exec`, `haco shell`, `haco run`, and `haco delete` use the Environment lifecycle | v0.1-v0.6 | unit/process coverage; real Incus acceptance pending |
| Workspace model / leases | canonical Workspace identity, persisted RO/RW leases, conflict prevention, and stale recovery are implemented | v0.1-v0.2 | unit/persistence/concurrency coverage |
| Incus Environment provider | Incus is the only Environment provider registered by the current build | v0.1+ | repository coverage; real supported-host acceptance pending |
| Client access | status, loopback forwarding, SSH preparation/revocation, and hardened public-key handling are implemented | v0.3 | unit/process coverage; real SSH acceptance host-dependent |
| Policy / Capability | fail-closed policy, approval, request correlation, and audit are implemented | v0.4 | unit/process/CLI coverage |
| Git / GitHub push capability | host-side brokered push keeps reusable credentials on the trusted side | v0.5 | unit/adversarial/real-git coverage |
| Agent / orchestrator integration | `haco run`, machine JSON, and external security events are implemented without moving orchestration into Core | v0.6 | unit/race/process coverage |
| Environment routing | provider-neutral routing seam remains implemented | v0.7 | router unit coverage |
| Remote / cloud providers | no remote/cloud Environment provider is registered or implemented in the current tree; the former EC2 runtime was removed while local/provider contracts stabilize | v0.7 historical slice | **cloud implementation is currently deferred**; history/design remain reference material |
| AWS capability / EBS helper | the former `aws.api` capability and EBS helper were removed together with the EC2 implementation | v0.7 historical slice | deferred; not current code reality |
| VS Code Client Adapter | `haco-vscode` prepares loopback SSH and launches standard VS Code Remote-SSH | v0.8 | helper coverage; real Windows/WSL + Incus + VS Code acceptance pending |
| Per-agent sandbox broker | opaque external sessions map to dedicated Environments using persisted ownership proof | v0.9 | ownership/persistence/release coverage |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release` uses the v0.9 broker and client-side private keys | v0.10 | implementation present; real Agent Host acceptance pending |
| Base identity / selection | provider-neutral Base identity resolves logical names to immutable revisions; CLI is `haco base list/inspect` and `create --base` | v0.11 | unit/fake-Incus coverage; real image-source acceptance pending |
| Resource budgets | CPU, memory, PID, and root-storage budgets are provider-neutral; Incus applies/read-backs finite limits before start | v0.12 | unit/fake-Incus coverage; real enforcement pending |
| Unsupported-provider resource behavior | finite requests fail before side effects when a provider cannot enforce them | v0.12 | wrapped-provider coverage |
| Managed sandbox network | managed `haco-sandbox0`, `haco-sandbox-egress`, and `haco-sandbox` are created/verified; broad/default fallback fails closed | v0.13 | unit/static coverage; real supported-Incus network acceptance pending |
| Git fetch plugin | `haco plugin git fetch <environment>` uses trusted Host `gh auth git-credential`, fixed refspecs, and rejects repository-controlled credential/transport redefinition | v0.14 | CLI/provider/security coverage |
| Optional OCI plugin boundary | OCI/container-specific implementation lives under `modules/plugin/oci`; `HACO_PLUGIN_OCI=nerdctl` or `HACO_PLUGIN_OCI=docker` explicitly composes it, while unset leaves Core independent of container tooling | cross-cutting | module/driver/CLI coverage; plugin-disabled Core path remains valid |
| OCI usage telemetry | `haco plugin oci seed sample` records latest OCI identity snapshots; `haco plugin oci seed recommend` ranks immutable identities over the 30-day window | v0.15 | `modules/plugin/oci` unit/persistence/sampling coverage |
| OCI Seed auto-selection | recommendation marks the deterministic top 10% of eligible immutable identities as `auto_promote=true` | v0.15 | deterministic unit coverage; physical Seed build pending |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` records trusted deletion/tombstone state and may explicitly extend removal to managed Environments | v0.16 | `modules/plugin/oci` deletion/tombstone/retry coverage |
| OCI deletion override | tombstones suppress recommendation/automatic re-promotion of the exact identity until explicitly superseded | v0.16 | persisted state/recommendation coverage |
| Docker compatibility | genuine Docker CLI/on-demand Engine design plus plugin-owned systemd socket/service packaging exists; `containerd + nerdctl` is a project-maintained optional OCI profile, not a Core requirement | v0.17 | plugin boundary/driver composition implemented; Base/Engine lifecycle/real-host acceptance still partial |
| Optional Local OCI Registry | Registry/proxy is optional and not required for ordinary Environment pull, telemetry, or Seed construction | v0.18 | planned / design only |
| OCI Seed Builder / COW | trusted Host acquisition, offline Seed Builder, immutable Seed publication/current pointer, conservative GC, and real Btrfs COW measurement remain incomplete | v0.19 | planned / acceptance pending |
| CI | Go matrix, vet, race, docs consistency, release packaging, local CI runner, and non-host-dependent E2Es are configured | cross-cutting | real-provider acceptance remains separate |

## Current implementation state

```text
v0.1  Secure Workspace Runtime
  -> v0.2 Workspace Lease
  -> v0.3 Client Access
  -> v0.4 Policy / Capability
  -> v0.5 GitHub push broker
  -> v0.6 Agent / Orchestrator surface
  -> v0.7 provider-neutral routing seam (cloud implementation deferred)
  -> v0.8 VS Code Client Adapter
  -> v0.9 Per-Agent binding broker
  -> v0.10 VS Code Agent Host adapter
  -> v0.11 immutable Base selection (`haco base`)
  -> v0.12 ResourceBudget
  -> v0.13 managed sandbox network
  -> v0.14 Git fetch plugin
  -> v0.15 OCI Seed telemetry / recommendation (`haco plugin oci`)
  -> v0.16 OCI image deletion / tombstone (`haco plugin oci`)
  -> v0.17 Docker compatibility foundation (partial)
  -> v0.18 optional Local Registry (planned)
  -> v0.19 Seed Builder / COW (planned)
```

## Cloud runtime deferral

The previous v0.7 EC2 runtime, host-side AWS capability, EBS replacement helper, and cloud-specific E2Es are intentionally absent from the active implementation tree. The provider-neutral routing boundary remains so a cloud adapter can be restored later without making cloud-specific behavior part of Core.

## Important boundaries

### v0.11 Base workflow

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

A logical Base resolves once to an immutable revision at creation. OCI/container images deliberately do not share the `haco base` namespace.

### v0.13 Managed Sandbox Network

Hacocoon owns a managed network/profile/ACL substrate and refuses silent fallback to broad/default Incus networking. Domain-aware authorization remains a higher-layer broker/proxy/plugin concern.

### v0.14 Git Fetch Plugin

Git fetch stays under `haco plugin git`. Reusable GitHub authentication remains on the trusted Host through `gh auth git-credential`; repository-controlled Git configuration cannot redefine the privileged transport path.

### v0.15 / v0.16 Optional OCI plugin

```text
HACO_PLUGIN_OCI=nerdctl haco plugin oci status
HACO_PLUGIN_OCI=docker haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
haco plugin oci image delete docker.io/library/node:24 --all-environments
```

With `HACO_PLUGIN_OCI` unset, OCI-specific commands are not composed and Core must not require or probe for `containerd`, `nerdctl`, Docker CLI, or Docker Engine.

v0.15 recommendation does not mean physical Seed build/publish is complete; v0.16 tombstones override automatic re-promotion of the exact deleted identity. Physical immutable Seed publication/GC belongs to v0.19.

### v0.17 Docker compatibility

Docker compatibility remains optional. `containerd + nerdctl` is the project-maintained OCI profile when that profile is selected, not the Hacocoon Core runtime. Plugin/package ownership and driver composition are implemented; official Base/Seed bake-in, complete on-demand Engine lifecycle, and real-host acceptance remain pending.

Plugin-owned systemd packaging lives at `modules/plugin/oci/packaging/systemd/`.

### v0.18 / v0.19 planned OCI infrastructure

A Local Registry is optional, not a prerequisite for ordinary OCI pulls or OCI Seed. v0.19 owns trusted Host acquisition, offline Seed Builder import, immutable Seed publication, revision pinning, and storage-driver COW behavior. Sharing one writable `/var/lib/containerd` across Environments is forbidden.

## Real-host acceptance

Repository tests are not substitutes for real-host acceptance. Still host-dependent are real Incus lifecycle/network/resource enforcement, Windows/WSL + VS Code, Agent Host routing, real image/private-registry combinations, Docker compatibility lifecycle, and OCI Seed/Btrfs behavior. Cloud acceptance is deferred until a concrete cloud adapter returns.

## Compatibility status

No milestone through v0.19 should be read as a promise that current CLI/API/state/config surfaces are frozen. Until an explicit stable compatibility milestone is declared, breaking changes may correct unsafe authority boundaries, ambiguous ownership, accidental provider coupling, or unnecessary complexity.
