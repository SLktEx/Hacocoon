# Implementation Status

Status date: 2026-08-30, after the v0.10 Agent Host adapter, v0.11 Base-selection, v0.12 resource-budget, v0.13 optional-OCI-plugin first slice, and the deliberate deferral of the previous EC2/AWS/EBS implementation slice.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Hacocoon is still **pre-1.0**.

| Area | Current repository reality | Release | Validation status |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace`, `haco exec`, `haco shell`, `haco delete` | v0.1 | repository coverage; real Incus acceptance pending |
| Workspace model / leases | canonical Workspace identity, persisted RO/RW leases, conflict prevention and recovery | v0.1-v0.2 | unit/concurrency/process coverage |
| Client access | status, loopback forwarding, SSH preparation/revocation | v0.3 | repository coverage; real Incus SSH host-dependent |
| Policy / Capability | fail-closed policy, human approval, request correlation and JSONL audit | v0.4 | unit/process/CLI coverage |
| Git / GitHub capability | brokered GitHub authority without exporting Host credentials | v0.5 | unit/adversarial/real-git coverage |
| Agent / orchestrator integration | `haco run`, stable machine JSON and external security event export | v0.6 | unit/race/process coverage |
| Environment routing | provider-neutral router remains; current build registers Incus only | v0.7 | router coverage |
| Remote / cloud providers | previous EC2 runtime, AWS capability and EBS helper are intentionally absent from the active tree | v0.7 historical slice | deferred; Git history/design retained for future restoration |
| VS Code Client Adapter | `haco-vscode` → loopback SSH → standard Remote-SSH | v0.8 | helper coverage; real Windows/WSL + Incus + VS Code acceptance pending |
| Per-agent sandbox broker | `internal/agenthost` binds opaque external sessions to dedicated Environments | v0.9 | allocation/idempotence/persistence/release coverage |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release` uses the v0.9 broker and loopback SSH | v0.10 | repository coverage; real Agent Host acceptance pending |
| Base identity / CLI | provider-neutral Base identity, immutable Incus revision pinning, `haco base list`, `haco base inspect`, `haco create --base` | v0.11 | unit/routing/fake-Incus coverage |
| Resource budget | finite/unlimited CPU, memory, PID and root-storage limits; Incus applies and verifies finite limits before start | v0.12 | unit/fake-Incus coverage; real enforcement pending |
| Optional OCI plugin boundary | OCI/container-specific implementation lives under `modules/plugin/oci`; Core does not own OCI telemetry, Seed deletion state, or Docker compatibility packaging | v0.13 | module boundary and focused unit coverage |
| OCI plugin composition | `HACO_PLUGIN_OCI=nerdctl` or `HACO_PLUGIN_OCI=docker` explicitly enables a driver; unset means no OCI plugin and no Core container-tool requirement | v0.13 | explicit driver parsing/selection coverage |
| OCI plugin namespace | container-specific management is under `haco plugin oci ...`; Base lifecycle remains `haco base ...` | v0.13 | CLI namespace routing coverage |
| OCI usage telemetry | `haco plugin oci seed sample` records image snapshots; `haco plugin oci seed recommend` refreshes stale samples and ranks immutable identities | v0.13 | unit coverage in `modules/plugin/oci`; real-host sampling pending |
| OCI Seed auto-selection | top 10% of eligible immutable identities, rounded up with minimum one, receive `auto_promote=true` | v0.13B | deterministic unit coverage; Seed build consumption pending |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` records plugin-owned tombstones and can extend deletion to managed Environments with `--all-environments` | v0.13C | focused plugin unit coverage; replacement Seed publish/old-Seed GC pending |
| Docker Engine compatibility packaging | socket/service templates live under `modules/plugin/oci/packaging/systemd`; Docker driver selection never grants authority over an arbitrary Host Docker daemon | v0.13 | systemd packaging validation; Base/Seed bake-in and real lifecycle acceptance pending |
| OCI Seed build/publish | offline Seed Builder, Environment harvesting, immutable Seed publication and real Btrfs/COW measurement are not complete | v0.13A | planned |
| Local OCI Registry | optional plugin infrastructure; not required for ordinary Environment pull, telemetry or Seed design | v0.13 | design only / optional optimization |
| CI | Go matrix, vet, race, docs consistency, release packaging, workflow trust checks and non-host-dependent E2Es are configured | cross-cutting | real-provider/profile acceptance remains separate |

## Current implementation state

```text
Workspace
  -> Environment lifecycle
  -> local Incus
  -> Workspace leases and client access
  -> Policy / Approval / Capability
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> provider-neutral routing seam (cloud implementation deferred)
  -> VS Code Client Adapter
  -> trusted external session -> Environment binding broker
  -> VS Code Remote Agent Host adapter
  -> logical Base -> immutable revision -> Environment
  -> explicit ResourceBudget -> provider enforcement
  -> optional OCI plugin -> explicit nerdctl/Docker driver
       -> telemetry / recommendation / deletion state
```

## Cloud runtime deferral

The previous v0.7 EC2 runtime, host-side AWS capability, EBS replacement helper and their cloud-specific E2Es are intentionally absent from the current implementation tree. This is a development-focus decision while local Runtime/Provider contracts are changing quickly. The provider-neutral routing seam remains so a cloud adapter can be restored later.

The previous EC2 implementation was experimental/default-off and had **real AWS acceptance pending**; it was never production-accepted.

## v0.8 client workflow

```bash
haco-vscode open .
haco-vscode delete .
```

## v0.9 per-agent binding model

```text
trusted client / VS Code integration
       -> opaque session ID
       -> internal/agenthost Broker
       -> agent-bindings.json ownership proof
       -> Environment service
       -> Incus
```

## v0.10 Agent Host adapter

```text
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

## v0.11 Base workflow

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

Operator mappings may use `HACO_INCUS_BASES_JSON`. Incus source vocabulary remains adapter-specific.

## v0.12 resource budgets

```text
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

## v0.13 OCI plugin workflow

```text
HACO_PLUGIN_OCI=nerdctl haco plugin oci status
HACO_PLUGIN_OCI=docker haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
haco plugin oci image delete docker.io/library/node:24 --all-environments
```

With `HACO_PLUGIN_OCI` unset, Core composition must not require or probe for `containerd`, `nerdctl`, Docker CLI, or Docker Engine.

See `00A_PLUGIN_ARCHITECTURE.md`, `OCI_RUNTIME_AND_DOCKER_COMPAT.md`, `13A_v0.13_OCI_SEED_AND_COW.md`, `13B_v0.13_SEED_AUTO_PROMOTION.md`, and `13C_v0.13_OCI_IMAGE_DELETION.md`.

## Compatibility status

Until an explicit stable compatibility milestone is declared, breaking changes may modify or replace CLI commands, helper binaries, persisted state, provider interfaces, Base/image lifecycle, capability/policy schemas, client/agent integration, resource-budget behavior and optional plugin profiles.

Compatibility must not be preserved at the cost of unsafe authority boundaries, ambiguous ownership, silent data loss, or unnecessary architectural complexity.
