# Implementation Status

Status date: 2026-08-30, after the v0.10 Agent Host adapter, v0.11 Base-selection, v0.12 resource-budget, initial v0.13 OCI telemetry/lifecycle implementation passes, and the temporary removal of the EC2/AWS/EBS implementation slice.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Versioned specifications are design/acceptance references; their existence does not imply full implementation.

Hacocoon is still **pre-1.0**. An implemented area does not mean its CLI/API/state/config surface is frozen, production support is guaranteed, or every real-provider/client acceptance test has passed.

| Area | Current repository reality | Release | Validation status |
|---|---|---:|---|
| Secure Workspace Runtime | public Environment path supports `haco create --workspace`, `haco exec`, `haco shell`, and `haco delete` | v0.1 | unit and process-boundary integration coverage exists; supported-host real Incus acceptance remains pending |
| Workspace model | `Workspace`, `Environment`, `ExecutionResult`, canonical external-path Workspace identity, and persisted Workspace leases are implemented | v0.1-v0.2 | unit, persistence, concurrency, and process-boundary coverage |
| Workspace lease safety | RO/RW leases, RW conflict prevention, stale-lease recovery, and process serialization are implemented | v0.2 | unit/concurrency/integration coverage |
| Incus Environment provider | concrete local Incus Environment implementation is the only Environment provider registered by the current build | v0.1+ | repository coverage exists; real Incus host acceptance remains pending |
| Client access | status, local-only port forwarding, connection listing/removal, SSH preparation/revocation, and hardened public-key handling are implemented | v0.3 | process/unit coverage; real Incus SSH acceptance host-dependent |
| Policy / Capability | fail-closed PolicyEvaluator, allow/deny/require-approval, human security approval, request correlation, and JSONL audit are implemented | v0.4 | unit/process integration and CLI capability coverage |
| Git / GitHub capability | host-side brokered GitHub push uses normalized repo/ref authority, exact source SHA, policy/approval, and force-with-lease semantics without exporting host credentials | v0.5 | unit, adversarial, real-git integration and CLI coverage |
| Agent / orchestrator integration | `haco run`, stable machine JSON output, and external security event export are implemented without moving orchestration/DAG/model selection into Hacocoon | v0.6 | unit/race/process integration coverage |
| Environment routing | provider-neutral Environment router exists; pre-v0.7 bare runtime refs remain backward-compatible as Incus refs; generic fake providers keep the routing seam exercised without a cloud implementation | v0.7 | router unit coverage |
| Remote / cloud Environment providers | no remote/cloud Environment provider is registered or implemented in the current tree; the previous experimental EC2 runtime was removed while the local contracts are changing rapidly | v0.7 historical slice | deferred; Git history/design remain available for a future adapter |
| AWS capability / EBS helper | the previous host-side `aws.api` capability, EBS replacement helper, and cloud-specific E2Es are removed from the active tree together with EC2 runtime support | v0.7 historical slice | deferred; the former status was **real AWS acceptance pending**, not production-accepted |
| VS Code Client Adapter | `haco-vscode` creates/reuses a matching Environment, prepares loopback SSH, writes adapter-owned SSH config, and launches standard VS Code Remote-SSH | v0.8 | helper coverage; real Windows/WSL + Incus + VS Code acceptance pending |
| Windows/WSL client bridge | under WSL, `haco-vscode` targets the desktop client's Windows `.ssh` configuration rather than WSL-only SSH config | v0.8 | code path implemented; real acceptance pending |
| Windows/WSL bootstrap | dedicated Hacocoon WSL instance setup with WSL2/systemd/Incus safety boundaries is implemented | v0.8 | static/syntax checks; real install/reboot acceptance pending |
| Client adapter boundary | IDE-specific launch/configuration remains outside Core | v0.8 | architecture/documentation boundary |
| Per-agent sandbox broker | `internal/agenthost` maps an opaque external session identity to a dedicated Environment through existing lifecycle and WorkspaceLease rules | v0.9 | unit coverage for allocation, idempotence, persisted binding proof and safe release |
| Agent binding state | trusted `agent-bindings.json` stores hashed session-to-Environment ownership proof | v0.9 | process-safe persistence; fault-injection acceptance pending |
| Agent control-plane separation | coding agents are not required to invoke `haco`; Incus/Hacocoon management authority stays trusted-side | v0.9 | architecture/test contract |
| VS Code Agent Host / AHP routing foundation | independently routable top-level agent sessions can be bound to dedicated Environments | v0.9 | broker foundation; real AHP routing pending |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release` uses the v0.9 broker and loopback SSH with client-side private keys | v0.10 | implementation present via PR #137; real Windows/WSL + Incus + VS Code Agent Host acceptance pending |
| Base identity model | Core exposes provider-neutral `BaseName`, `BaseRevision`, `BaseRef`, and persists the exact Base revision selected for an Environment | v0.11 | unit/routing/persistence coverage |
| Incus Base resolution | logical Base names map to adapter-owned Incus sources and resolve to validated immutable fingerprints before creation | v0.11 | alias movement coverage; real Incus image-remote acceptance pending |
| Base CLI | `haco base list`, `haco base inspect <base>`, and `haco create --base <base> --workspace <path> <environment>` are implemented; the old ambiguous `haco image ...` namespace is removed | v0.11 | namespace routing tests plus existing Base selection/inspect coverage |
| Custom Base mapping | host/operator may add logical custom Base mappings through `HACO_INCUS_BASES_JSON`; the `haco/` namespace is reserved and unsafe inputs fail closed | v0.11 | adversarial coverage; custom build/import/history/rollback/GC future |
| Resource budget model | Core exposes provider-neutral finite/unlimited CPU, memory-byte, PID, and root-byte limits; effective budgets are persisted | v0.12 | normalization/bounds/persistence coverage |
| Resource CLI | `haco create` and `haco run` accept `--cpu`, `--memory`, `--pids`, and `--root-size` | v0.12 | parser/fake-Incus coverage |
| Incus resource enforcement | finite CPU/memory/PID/root-disk limits are applied and read back before `start`; mismatch or failure aborts creation | v0.12 | unit/fake-Incus coverage; real enforcement pending |
| Unsupported-provider resource behavior | providers that cannot enforce requested finite budgets reject before side effects; this remains a provider-neutral contract independent of any EC2 implementation | v0.12 | wrapped-provider coverage |
| OCI plugin namespace | OCI/containerd/nerdctl-specific management is separated from Base lifecycle under `haco plugin oci ...` | v0.13 | CLI namespace routing coverage; dynamic plugin loading is not implied |
| OCI usage telemetry | `haco plugin oci seed sample` records latest OCI image identity snapshots per Environment; `haco plugin oci seed recommend` refreshes stale samples and ranks immutable identities over a 30-day window | v0.13 | unit coverage in `internal/seedstats`; real-host sampling acceptance pending |
| OCI Seed auto-selection | recommendation marks the top 10% of eligible immutable identities, rounded up with minimum one, as `auto_promote=true` | v0.13 | deterministic unit coverage; Seed build consumption still pending |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` removes the Host Seed-cache reference and records a deletion tombstone; `--all-environments` explicitly extends deletion to managed Environments without `--force` | v0.13 | focused unit coverage; replacement Seed publish/old-Seed GC pending |
| OCI deletion override | deletion tombstones suppress recommendation/automatic re-promotion of the exact identity until an explicit future Seed override clears/supersedes the tombstone | v0.13 | persisted state v2 and recommendation coverage |
| OCI Seed build/publish | offline Seed Builder, Environment harvesting, immutable Seed publish, current-pointer movement, and real Btrfs COW measurement are not yet complete | v0.13 | planned / acceptance pending |
| Local OCI Registry | Registry/proxy is optional and not required for ordinary Environment pull, telemetry, or Seed design | v0.13 | design only / optional optimization |
| Btrfs / raw / QCOW2 historical storage | historical local storage implementation remains in the repository | historical / provider detail | not part of the current Core Environment model |
| CI | Go matrix, vet, race, docs consistency, release packaging and non-host-dependent E2Es are configured | cross-cutting | GitHub Actions is currently failing jobs before steps execute due a repository-wide Actions-side issue; real-provider acceptance remains separate |

## Current implementation state

The stable earlier progression remains contiguous through v0.12, with partial v0.13 OCI slices now also present:

```text
Workspace
  -> Environment lifecycle
  -> local Incus
  -> Workspace leases and client access
  -> Policy / Approval / Capability boundary
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> provider-neutral routing seam (cloud implementation deferred)
  -> thin Client Adapter layer, starting with VS Code Remote-SSH
  -> trusted external agent session -> persisted Environment binding broker
  -> VS Code Remote Agent Host adapter
  -> logical Base -> immutable revision -> Environment
  -> explicit ResourceBudget -> provider enforcement before Environment access
  -> optional OCI plugin -> telemetry / recommendation / deletion state
```

The v0.9 broker does not introduce an agent-visible management CLI. A trusted integration supplies an opaque session identity and Workspace; the broker selects/creates the Environment and persists ownership proof separately.

v0.10 is implemented through `haco-agent-host`. It prepares a v0.9-bound Environment as a managed loopback SSH target while leaving VS Code in control of the Agent Host UI/protocol.

v0.11 resolves a selected logical Base once at Environment creation time, records immutable `BaseRevision`, and initializes Incus from the pinned fingerprint. Its CLI is now explicitly `haco base ...`, not the generic `haco image ...` name.

v0.12 resolves every creation request to an explicit effective `ResourceBudget`. Incus finite limits are configured and verified before Environment start; unsupported finite requests fail closed.

The initial v0.13 OCI slice is deliberately an optional plugin namespace. `haco plugin oci seed recommend` does not mean Seed build/publish is complete: telemetry, ranking, top-10% selection, and deletion/tombstone state exist, while Host-cache harvesting, offline builder publication, immutable Seed pointer movement, GC and real Btrfs COW acceptance remain pending.

The Windows/WSL bootstrap remains a host setup helper, not a new Core lifecycle.

## Cloud runtime deferral

The previous v0.7 EC2 runtime, host-side AWS capability, EBS replacement helper, and their cloud-specific E2Es are intentionally absent from the current implementation tree.

This is a development-focus decision, not a statement that the architecture can never support cloud providers. Hacocoon is changing quickly enough that keeping cloud-specific code synchronized without regular real-cloud acceptance testing creates maintenance work without reliable validation. The provider-neutral routing boundary remains specifically so a cloud adapter can be restored later after the local contracts stabilize.

The v0.7 design documents describe the historical implementation direction. In that historical state the EC2 provider was **experimental and disabled by default** and real AWS acceptance pending. Those documents and Git history remain reference material, not current implementation status.

Real Incus, Windows/WSL + VS Code Remote-SSH, per-agent Agent Host routing, real Base sources, real resource enforcement, and OCI Seed build/Btrfs behavior require suitable hosts. Repository tests are not substitutes for those checks.

## v0.8 client workflow

```bash
haco-vscode open .
```

Cleanup:

```bash
haco-vscode delete .
```

## v0.9 per-agent binding model

```text
trusted client / VS Code integration
       -> opaque session ID
       -> internal/agenthost Broker
       -> persisted ownership proof
       -> Environment service
       -> Incus
```

## v0.10 Agent Host adapter

```text
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

See `10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`.

## v0.11 Base workflow

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 remains recorded on revision A
```

Official logical Bases are provided by the Incus adapter and operator mappings may use `HACO_INCUS_BASES_JSON`. Incus alias/remote/fingerprint vocabulary remains an adapter detail.

See `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` and `BASE_IMAGES.md`.

## v0.12 resource budgets

```text
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

See `12_v0.12_SANDBOX_RESOURCE_LIMITS.md`.

## v0.13 OCI plugin workflow

```text
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
haco plugin oci image delete docker.io/library/node:24 --all-environments
```

The CLI namespace separates OCI/container-image lifecycle from Hacocoon/Incus Base-image lifecycle. The old `haco image ...` command is intentionally removed rather than kept as a pre-1.0 compatibility alias.

See `13A_v0.13_OCI_SEED_AND_COW.md`, `13B_v0.13_SEED_AUTO_PROMOTION.md`, and `13C_v0.13_OCI_IMAGE_DELETION.md`.

## Compatibility status

Until an explicit stable compatibility milestone is declared, breaking changes may modify or replace CLI commands, helper binaries, persisted state, provider interfaces, Base/image lifecycle, capability/policy schemas, client/agent integration, host bootstrap behavior, resource-budget design, and runtime behavior.

Compatibility should not be preserved at the cost of an unsafe authority boundary, ambiguous ownership, silent data loss, or unnecessary architectural complexity. Material breaking changes should still be explicit, tested, and documented.
