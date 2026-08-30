# Implementation Status

Status date: 2026-08-30, after the v0.10 Agent Host adapter, v0.11 Base-selection, and v0.12 resource-budget implementation passes.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Versioned specifications are design/acceptance references; their existence does not imply implementation.

Hacocoon is still **pre-1.0**. An implemented area does not mean its CLI/API/state/config surface is frozen, production support is guaranteed, or every real-provider/client acceptance test has passed.

The current numbering keeps implemented milestones contiguous through **v0.12**.

| Area | Current repository reality | Release | Validation status |
|---|---|---:|---|
| Secure Workspace Runtime | public Environment path supports `haco create --workspace`, `haco exec`, `haco shell`, and `haco delete` | v0.1 | unit and process-boundary integration pass; supported-host real Incus acceptance remains pending |
| Workspace model | `Workspace`, `Environment`, `ExecutionResult`, canonical external-path Workspace identity, and persisted Workspace leases are implemented | v0.1-v0.2 | unit, persistence, concurrency, and process-boundary tests pass |
| Workspace lease safety | RO/RW leases, RW conflict prevention, stale-lease recovery, and process serialization are implemented | v0.2 | unit/concurrency/integration tests pass |
| Incus Environment provider | concrete local Incus Environment implementation remains the default runtime | v0.1+ | unit/process tests pass; real Incus host acceptance remains pending |
| Client access | status, local-only port forwarding, connection listing/removal, SSH preparation/revocation, and hardened public-key handling are implemented | v0.3 | unit/process integration pass; real Incus SSH acceptance remains host-dependent |
| Policy / Capability | fail-closed PolicyEvaluator, allow/deny/require-approval, human security approval, request correlation, and JSONL audit are implemented | v0.4 | unit/process integration and actual CLI capability E2E pass |
| Git / GitHub capability | host-side brokered GitHub push uses normalized repo/ref authority, exact source SHA, policy/approval, and force-with-lease semantics without exporting host credentials | v0.5 | unit, adversarial tests, real-git integration, and actual CLI E2E pass |
| Agent / orchestrator integration | `haco run`, stable machine JSON output, and external security event export are implemented without moving orchestration/DAG/model selection into Hacocoon | v0.6 | unit/race/process integration and actual CLI E2E pass |
| Environment routing | provider-neutral Environment router exists; pre-v0.7 bare runtime refs remain backward-compatible as Incus refs | v0.7 | router unit tests pass |
| EC2 Environment provider | experimental S3-staged / SSM-driven EC2 Environment provider exists; EC2 is disabled by default and requires both provider selection and explicit Hacocoon-owned opt-in | v0.7 | unit, fake-`aws` process integration, and actual `haco` fake-AWS E2E pass; real AWS acceptance pending |
| Experimental EC2 gate | `HACO_RUNTIME_PROVIDER=runtime.ec2` does not enable EC2 alone; `HACO_EXPERIMENTAL_EC2=1` is also required, and disabled paths fail before AWS activity | v0.7 | actual binary E2E verifies zero fake-AWS calls on the disabled path |
| AWS capability | narrow host-side `aws.api` read capability is mediated through the existing Policy/Approval/Audit path and generic capability CLI | v0.7 | unit/process integration and fake-AWS CLI E2E pass; real AWS acceptance pending |
| EBS replacement | adapter-owned replacement/migration flow exists for shrink-like operations; no in-place EBS shrink and no automatic source-volume deletion | v0.7 | unit and fake-AWS process integration cover preflight, migration, verification, cleanup, and recovery-required transitions |
| VS Code Client Adapter | separate `haco-vscode` binary creates/reuses a matching Environment, prepares existing loopback SSH access, writes isolated adapter-owned SSH host configuration, and launches standard VS Code Remote-SSH to `/workspace` | v0.8 | helper unit coverage added; real Windows/WSL + Incus + VS Code Remote-SSH acceptance remains pending |
| Windows/WSL client bridge | when run under WSL, `haco-vscode` resolves the Windows user profile and targets the desktop-client `.ssh` configuration rather than WSL-only SSH config | v0.8 | code path implemented; real Windows/WSL acceptance pending |
| Windows/WSL bootstrap | standalone/source bootstrap creates or reuses only a dedicated named WSL instance (`Hacocoon` by default), enforces WSL 2 for that dedicated instance, installs `systemd`/`systemd-sysv`, preserves unrelated `/etc/wsl.conf` keys while enforcing `[boot] systemd=true`, restarts only the dedicated instance when required, verifies systemd as PID 1, then starts Incus; unrelated WSL distributions/global defaults remain untouched and `incus-admin` requires explicit opt-in | v0.8 | PowerShell/shell syntax and static WSL2/systemd contract checked in CI; real Windows install/reboot/WSL2 conversion/systemd/Incus acceptance remains pending |
| Client adapter boundary | IDE-specific launch/configuration remains outside Core; Core does not depend on VS Code, Daintree, JetBrains, or client-native configuration | v0.8 | architecture/documentation contract plus separate adapter binary |
| Per-agent sandbox broker | `internal/agenthost` maps an opaque external session identity to a dedicated Environment through the existing Environment/WorkspaceLease lifecycle | v0.9 | unit coverage for dedicated allocation, idempotence, rebinding rejection, persisted restart lookup, raw-ID non-disclosure, deterministic-name collision refusal, and proof-required release |
| Agent binding state | session-to-Environment ownership proof is stored separately in trusted `agent-bindings.json`; raw external session IDs are hashed before persistence | v0.9 | process-safe Linux file lock plus atomic/fsync-backed writes; real-host crash/fault-injection acceptance pending |
| Agent control-plane separation | coding agents are not required to invoke `haco`; Incus/Hacocoon management authority stays on the trusted side of the Environment boundary | v0.9 | repository architecture/test contract; real-host adversarial validation pending |
| VS Code Agent Host / AHP routing foundation | target architecture places each independently routable top-level agent session/Agent Host in its assigned Environment | v0.9 | broker/control-plane foundation implemented; real VS Code Agent Host/AHP + Incus end-to-end routing acceptance pending |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release` uses the v0.9 broker, hashed session-derived SSH aliases, loopback-only SSH access, client-side private keys, managed SSH config fragments, and optional `code --agents` launch | v0.10 | implemented on `main` via PR #137; unit/release CI pass; real Windows/WSL + Incus + VS Code Agent Host acceptance remains pending |
| Base identity model | Core exposes provider-neutral `BaseName`, `BaseRevision`, `BaseRef`, and persists the exact Base revision selected for an Environment | v0.11 | unit/routing/persistence-oriented coverage; concrete compatibility remains pre-1.0 |
| Incus Base resolution | logical Base names map to adapter-owned Incus sources; creation resolves the mutable source to a validated immutable fingerprint and initializes from the pinned fingerprint | v0.11 | unit tests prove alias movement cannot retarget the previously resolved revision; real Incus image-remote acceptance pending |
| Base CLI | `haco image list`, `haco image inspect <base>`, and `haco create --base <base> --workspace <path> <environment>` are implemented; `status --json` exposes persisted Base metadata | v0.11 | unit and fake-Incus E2E cover selection, inspect, pinning, and persisted revision |
| Custom Base mapping | host/operator may add logical custom Base mappings through `HACO_INCUS_BASES_JSON`; the `haco/` namespace is reserved and unsafe Base/source/fingerprint inputs fail closed | v0.11 | adversarial unit coverage; custom build/import/history/rollback/GC remain future work |
| Resource budget model | Core exposes provider-neutral finite/unlimited CPU, memory-byte, PID, and root-byte limits; omission resolves to an explicit unlimited effective budget and the effective budget is persisted on Environment metadata | v0.12 | unit coverage for normalization, bounds, invalid modes/values, and persistence boundary |
| Resource CLI | `haco create` and `haco run` accept `--cpu`, `--memory`, `--pids`, and `--root-size`; byte values use strict `B`/`KiB`/`MiB`/`GiB`/`TiB` units or `unlimited` | v0.12 | parser unit coverage plus fake-Incus CLI E2E |
| Incus resource enforcement | finite CPU/memory/PID/root-disk limits are applied and read back before `start`; a mismatch or apply/verify failure aborts creation and enters normal cleanup/recovery handling | v0.12 | unit tests cover ordering, verification mismatch, and cleanup; fake-Incus E2E covers persisted values and provider-native commands; real Incus enforcement pending |
| Unsupported-provider resource behavior | finite resource requests to providers that do not implement them fail closed before provider side effects; experimental EC2 is wrapped by this boundary | v0.12 | unit test proves wrapped provider create is not called for finite budgets; real AWS remains experimental/pending |
| Btrfs / raw / QCOW2 historical storage | historical local storage implementation remains in the repository | historical / provider detail | not part of the current Core Environment model and not a compatibility commitment |
| CI | Go version matrix tests, `go vet`, race detector, docs consistency, bootstrap syntax, release packaging, workflow trust policy, and existing non-host-dependent E2Es are enabled | cross-cutting | real-provider/client acceptance remains separate |

## Current implementation state

The implemented progression is now contiguous through v0.12:

```text
Workspace
  -> Environment lifecycle
  -> local Incus by default
  -> Workspace leases and client access
  -> Policy / Approval / Capability boundary
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> experimental remote EC2 provider and AWS capability
  -> thin Client Adapter layer, starting with VS Code Remote-SSH
  -> dedicated Windows/WSL 2 + systemd bootstrap helper outside Core
  -> trusted external agent session -> persisted Environment binding broker
  -> VS Code Remote Agent Host adapter
  -> logical Base -> immutable revision -> Environment
  -> explicit ResourceBudget -> provider enforcement before Environment access
```

The v0.9 broker does not introduce an agent-visible management CLI. A trusted integration supplies an opaque session identity and Workspace; the broker selects/creates the Environment and persists ownership proof separately. A deterministic Environment name is not sufficient proof: without a matching persisted binding, Acquire refuses adoption and Release refuses deletion.

Parallel write-capable agent sessions still need distinct canonical Workspace paths, normally separate Git worktrees. Existing WorkspaceLease conflict prevention is not weakened for multi-agent convenience.

v0.10 is implemented through `haco-agent-host`. It prepares a v0.9-bound Environment as a managed loopback SSH target for the VS Code Agents window, keeps the private key on the client side, and leaves VS Code in control of the Agent Host UI/protocol. Real-host Agent Host/AHP acceptance remains pending.

v0.11 resolves a selected logical Base once at Environment creation time, records the immutable `BaseRevision`, and initializes Incus from the pinned fingerprint rather than the mutable alias. Changing a logical Base source later affects future Environment creation only; it does not rewrite persisted Base identity for an existing Environment.

The first v0.11 slice deliberately does not claim custom image build/import, revision history, rollback, or garbage collection. Those operations need explicit reference/deletion safety before they should be added.

v0.12 resolves every creation request to an explicit effective `ResourceBudget`. Omitted dimensions become explicit `unlimited`; finite dimensions are bounded and persisted. For Incus, finite limits are configured and verified before the Environment starts. A provider that cannot enforce a requested finite limit must reject the request rather than silently ignore it. The experimental EC2 path currently takes that fail-closed route for finite budgets.

The Windows/WSL bootstrap remains a host setup helper, not a new Core lifecycle. It reserves a dedicated WSL instance for Hacocoon, may convert only that Hacocoon-owned instance from WSL 1 to WSL 2, installs/enables systemd, restarts only the named instance when required, verifies systemd as PID 1, and leaves unrelated WSL distributions/global defaults untouched. `incus-admin` is never granted silently. See `WINDOWS_WSL_BOOTSTRAP.md`.

The v0.7 EC2 provider remains **experimental and disabled by default**. Real AWS/EC2/SSM/EBS acceptance remains pending.

Real Incus, Windows/WSL + VS Code Remote-SSH, v0.9/v0.10 per-agent Agent Host routing, v0.11 real image sources, v0.12 real resource enforcement, and cloud acceptance require suitable hosts. Unit tests, fake-provider E2Es, race checks, vet, build, script syntax, and repository CI are not substitutes for those checks.

## v0.8 client workflow

```bash
haco-vscode open .
```

Conceptually:

```text
local Workspace
  -> create/reuse Hacocoon Environment
  -> prepare loopback-only SSH
  -> create adapter-owned SSH host entry
  -> code --remote ssh-remote+<alias> /workspace
```

Cleanup:

```bash
haco-vscode delete .
```

## v0.9 per-agent binding model

```text
trusted VS Code/AHP integration / trusted client
                 |
          opaque session ID
                 |
       internal/agenthost Broker
                 |
       persisted ownership proof
                 |
       existing Environment service
                 |
              Incus
```

Release accepts a session identity, not an arbitrary Environment name. The initial ownership unit is an independently routable top-level session; hidden harness-internal subagents may share the parent's Environment unless the client exposes a separate routable lifecycle.

## v0.10 Agent Host adapter

```text
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

See `10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`.

## v0.11 Base workflow

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 remains recorded on revision A
```

For the first implementation slice, official logical Bases are provided by the Incus adapter and host/operator custom mappings may be supplied with `HACO_INCUS_BASES_JSON`. Incus alias/remote/fingerprint vocabulary remains an adapter detail rather than a Core architecture contract.

See `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` and `BASE_IMAGES.md`.

## v0.12 resource budgets

```text
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

The first slice supports creation-time CPU, memory, PID, and root-disk budgets. It does not add live resizing, aggregate host scheduling, or Workspace quota management. `status` and `status --json` expose the persisted effective budget using provider-neutral fields.

See `12_v0.12_SANDBOX_RESOURCE_LIMITS.md`.

## Compatibility status

No versioned design or implementation row through v0.12 should be read as a promise that the current concrete interface will remain unchanged.

Until an explicit stable compatibility milestone is declared, breaking changes may modify or replace CLI commands, helper binaries, persisted state, provider interfaces, Base/image lifecycle, capability/policy schemas, client/agent integration, host bootstrap behavior, resource-budget design, and experimental runtime behavior.

Compatibility should not be preserved at the cost of an unsafe authority boundary, ambiguous ownership, silent data loss, or unnecessary architectural complexity. Material breaking changes should still be explicit, tested, and documented.
