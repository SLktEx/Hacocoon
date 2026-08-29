# Implementation Status

Status date: 2026-08-29, after the v0.11 VS Code Remote Agent Host Adapter implementation pass.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Release specifications under `docs/01_...` through `docs/11_...` are versioned design references. The v0.9 Base Images & Custom Environments specification remains a design contract whose implementation is pending; v0.10 and v0.11 add implemented agent-session and VS Code adapter foundations on the existing Environment/client-access lifecycle.

Hacocoon is still **pre-1.0**. An implemented area does not mean its CLI/API/state/config surface is frozen, production support is guaranteed, or every real-provider/client acceptance test has passed.

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
| Windows/WSL client bridge | under WSL, client adapters resolve the Windows user profile and target the desktop-client `.ssh` configuration rather than WSL-only SSH config | v0.8+ | code path implemented; real Windows/WSL acceptance pending |
| Windows/WSL bootstrap | standalone/source bootstrap creates or reuses only a dedicated named WSL instance (`Hacocoon` by default), enforces WSL 2 and systemd for that dedicated instance, restarts only it when required, then starts Incus; unrelated WSL distributions/global defaults remain untouched and `incus-admin` requires explicit opt-in | v0.8 | PowerShell/shell syntax and static WSL2/systemd contract checked in CI; real Windows install/reboot/WSL2/systemd/Incus acceptance remains pending |
| Client adapter boundary | IDE-specific launch/configuration remains outside Core; Core does not depend on VS Code, Daintree, JetBrains, or client-native configuration | v0.8+ | architecture/documentation contract plus separate adapter binaries |
| Base Images & Custom Environments | explicit v0.9 roadmap contract defines logical Base names, immutable Base revisions, Incus fingerprint pinning behind the adapter, custom-image trust boundaries, and safe reference/deletion semantics | v0.9 | **design only; implementation pending** — `haco image` / `haco create --base` must not be reported as implemented yet |
| Per-agent sandbox broker | `internal/agenthost` maps an opaque external session identity to a dedicated Environment through the existing Environment/WorkspaceLease lifecycle | v0.10 | unit coverage for dedicated allocation, idempotence, rebinding rejection, persisted restart lookup, raw-ID non-disclosure, deterministic-name collision refusal, and proof-required release |
| Agent binding state | session-to-Environment ownership proof is stored separately in trusted `agent-bindings.json`; raw external session IDs are hashed before persistence | v0.10 | process-safe Linux file lock plus atomic/fsync-backed writes; real-host crash/fault-injection acceptance pending |
| Agent control-plane separation | coding agents are not required to invoke `haco`; Incus/Hacocoon management authority stays on the trusted side of the Environment boundary | v0.10 | repository architecture/test contract; real-host adversarial validation pending |
| VS Code Remote Agent Host adapter | `haco-agent-host prepare/release` composes v0.10 binding with existing loopback SSH client access, hashed SSH aliases, Windows client-profile handling, connection reuse/rotation, and optional `code --agents` launch | v0.11 | helper unit coverage plus normal Go/CI gates; real Agents-window remote-SSH acceptance pending |
| Agent Host / AHP ownership | Hacocoon does not implement AHP; current VS Code is expected to install/start its remote CLI/Agent Host after the user selects the prepared SSH target | v0.11 | external-client behavior documented; real current-VS-Code acceptance pending |
| v0.11 isolation unit | one trusted `--session` slot maps to one v0.10 Environment; multiple VS Code sessions deliberately created through the same prepared alias may share that Environment | v0.11 | explicit contract; automatic VS Code-internal session-ID routing is not claimed |
| Release packaging | `haco`, `haco-vscode`, and `haco-agent-host` are included in Linux amd64/arm64 release archives and the Linux installer | v0.11 | GoReleaser dry-run and archive-content checks enabled in CI |
| Btrfs / raw / QCOW2 historical storage | historical local storage implementation remains in the repository | historical / provider detail | not part of the current Core Environment model and not a compatibility commitment |
| CI | Go version matrix tests, `go vet`, race detector, workflow trust policy, docs consistency, bootstrap syntax, release packaging, and existing non-host-dependent E2Es are enabled | cross-cutting | v0.11 PR CI must pass before merge; real-provider/client acceptance remains separate |

## Current implementation state

The implemented progression includes v0.1-v0.8 plus the v0.10-v0.11 agent integration foundations:

```text
Workspace
  -> Environment lifecycle
  -> local Incus by default
  -> Workspace leases and client access
  -> Policy / Approval / Capability boundary
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> experimental remote EC2 provider and AWS capability
  -> normal VS Code Remote-SSH adapter
  -> dedicated Windows/WSL 2 + systemd bootstrap
  -> trusted external agent session -> persisted Environment binding
  -> VS Code Agents-window remote-SSH adapter
```

The numbering is intentionally not an implementation-completeness shortcut: **v0.9 Base Images & Custom Environments remains design-only and implementation-pending**, while the independent v0.10 and v0.11 agent-integration foundations are implemented as additive code.

The v0.10 broker does not introduce an agent-visible management CLI. A trusted integration supplies an opaque session identity and Workspace; the broker selects/creates the Environment and persists ownership proof separately. A deterministic Environment name is not sufficient proof: without a matching persisted binding, Acquire refuses adoption and Release refuses deletion.

v0.11 adds `haco-agent-host`, which prepares a loopback-only SSH target for that v0.10 binding. It validates the client private/public key pair before access setup, passes only the public key through the existing Hacocoon SSH boundary, writes a hashed Hacocoon-owned SSH alias, and can launch the VS Code Agents window with `code --agents`.

The adapter intentionally does **not** implement AHP or invoke an internal Agent Host protocol itself. The trusted user/client selects the prepared alias using the Agents window remote-SSH flow, and VS Code owns its remote CLI/Agent Host lifecycle.

The v0.11 isolation unit is one prepared Hacocoon `--session` slot. Hacocoon does not currently receive VS Code's internal top-level agent-session UUID automatically. Reusing one prepared alias for multiple VS Code sessions means those sessions share the same Environment. Independent write-capable work therefore still needs separate session slots and normally separate Git worktrees.

`prepare` setup failures do not implicitly release the v0.10 binding; only newly prepared SSH forwarding is rolled back. This avoids a concurrent-prepare race deleting another trusted caller's binding. Explicit `release` is the destructive Environment cleanup path.

The Windows/WSL bootstrap remains a host setup helper, not a new Core lifecycle. It reserves a dedicated WSL instance for Hacocoon, may convert only that Hacocoon-owned instance from WSL 1 to WSL 2, installs/enables systemd, restarts only the named instance when required, verifies systemd as PID 1, and leaves unrelated WSL distributions/global defaults untouched. `incus-admin` is never granted silently.

The v0.7 EC2 provider remains **experimental and disabled by default**. Real AWS/EC2/SSM/EBS acceptance remains pending.

Real Incus, Windows/WSL + VS Code Remote-SSH, v0.11 Agents-window remote-SSH/Agent Host behavior, and cloud acceptance require suitable hosts. Unit tests, fake-provider E2Es, race checks, vet, build, script syntax, release dry-runs, and repository CI are not substitutes for those checks.

## v0.8 normal VS Code workflow

```bash
haco-vscode open .
```

## v0.9 planned Base workflow

The v0.9 design intends approximately:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

This command surface is **not implemented yet and is not frozen**.

## v0.10 per-agent binding model

```text
trusted client
   -> opaque session ID
   -> internal/agenthost Broker
   -> persisted ownership proof
   -> Environment
```

## v0.11 VS Code Agents-window workflow

Prepare one remote slot:

```bash
haco-agent-host prepare --session task-a /path/to/worktree-a
```

The command prints a hashed SSH alias and can open the Agents window. Select:

```text
New -> Remote -> SSH -> <printed alias>
```

For another isolated write-capable agent, use another session identity and another worktree:

```bash
haco-agent-host prepare --session task-b /path/to/worktree-b
```

Release explicitly:

```bash
haco-agent-host release --session task-a
```

See `11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`.

## Compatibility status

No v0.1-v0.11 design or implementation row should be read as a promise that the current concrete interface will remain unchanged.

Until an explicit stable compatibility milestone is declared, breaking changes may modify or replace CLI commands, helper binaries, persisted state, provider interfaces, Base/image lifecycle, capability/policy schemas, client/agent integration, host bootstrap behavior, and experimental runtime behavior.

Compatibility should not be preserved at the cost of an unsafe authority boundary, ambiguous ownership, silent data loss, or unnecessary architectural complexity. Material breaking changes should still be explicit, tested, and documented.
