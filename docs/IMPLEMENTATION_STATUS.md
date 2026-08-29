# Implementation Status

Status date: 2026-08-30, after the per-agent sandbox broker implementation pass and roadmap renumbering.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Versioned specifications are design/acceptance references; their existence does not imply implementation.

Hacocoon is still **pre-1.0**. An implemented area does not mean its CLI/API/state/config surface is frozen, production support is guaranteed, or every real-provider/client acceptance test has passed.

The current numbering keeps implemented milestones contiguous through **v0.9**. v0.10 is the active VS Code Remote Agent Host Adapter integration candidate. v0.11 Base Images and v0.12 Resource Limits remain design-only.

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
| VS Code Remote Agent Host Adapter | `haco-agent-host` integration is being developed in PR #111 on top of the v0.9 broker | v0.10 | **not yet on `main`**; branch/rebase/real-host acceptance remain pending |
| Base Images & Custom Environments | roadmap contract defines logical Base names, immutable Base revisions, Incus fingerprint pinning behind the adapter, custom-image trust boundaries, and safe reference/deletion semantics | v0.11 | **design only; implementation pending** — `haco image` / `haco create --base` must not be reported as implemented yet |
| Sandbox Resource Limits | roadmap contract defines provider-neutral CPU/memory/PID/root-storage budgets and fail-closed provider enforcement | v0.12 | **design only; implementation pending** |
| Btrfs / raw / QCOW2 historical storage | historical local storage implementation remains in the repository | historical / provider detail | not part of the current Core Environment model and not a compatibility commitment |
| CI | Go version matrix tests, `go vet`, race detector, docs consistency, bootstrap syntax, release packaging, workflow trust policy, and existing non-host-dependent E2Es are enabled | cross-cutting | real-provider/client acceptance remains separate |

## Current implementation state

The implemented progression is now contiguous through v0.9:

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
```

The v0.9 broker does not introduce an agent-visible management CLI. A trusted integration supplies an opaque session identity and Workspace; the broker selects/creates the Environment and persists ownership proof separately. A deterministic Environment name is not sufficient proof: without a matching persisted binding, Acquire refuses adoption and Release refuses deletion.

Parallel write-capable agent sessions still need distinct canonical Workspace paths, normally separate Git worktrees. Existing WorkspaceLease conflict prevention is not weakened for multi-agent convenience.

The preferred VS Code direction is the standalone Agent Host / Agent Host Protocol architecture, with the execution host next to the assigned Workspace inside the Environment. PR #111 is the v0.10 Remote Agent Host Adapter integration candidate; it is not part of `main` until merged.

The Windows/WSL bootstrap remains a host setup helper, not a new Core lifecycle. It reserves a dedicated WSL instance for Hacocoon, may convert only that Hacocoon-owned instance from WSL 1 to WSL 2, installs/enables systemd, restarts only the named instance when required, verifies systemd as PID 1, and leaves unrelated WSL distributions/global defaults untouched. `incus-admin` is never granted silently. See `WINDOWS_WSL_BOOTSTRAP.md`.

The v0.7 EC2 provider remains **experimental and disabled by default**. Real AWS/EC2/SSM/EBS acceptance remains pending.

Real Incus, Windows/WSL + VS Code Remote-SSH, v0.9 per-agent routing, v0.10 Agent Host Adapter, and cloud acceptance require suitable hosts. Unit tests, fake-provider E2Es, race checks, vet, build, script syntax, and repository CI are not substitutes for those checks.

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

For Windows host setup, the repository provides:

```powershell
.\install-windows.ps1
.\scripts\bootstrap-windows.ps1
```

Both paths enforce a dedicated Hacocoon WSL 2 instance with systemd as PID 1 before the local Incus path is considered ready.

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

The v0.9 path composes with v0.11 Base selection when that implementation exists; it does not bypass or redefine the Base contract.

See `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`.

## v0.10 active Agent Host adapter

PR #111 is the active v0.10 integration candidate. It adds the client-side/trusted adapter needed to prepare a v0.9-bound Environment as a loopback-only SSH target for the VS Code Agents window while keeping private keys on the client side.

This functionality must not be reported as part of `main` until the PR is rebased, renumbered consistently, merged, and its host-dependent acceptance boundary is documented.

## v0.11 planned Base workflow

The v0.11 design intends approximately:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

This command surface is **not implemented yet and is not frozen**.

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 remains on revision A
```

See `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` and `BASE_IMAGES.md`.

## v0.12 planned resource budgets

The v0.12 design adds provider-neutral creation-time CPU, memory, PID, and root-storage budgets. Requested limits that a provider cannot enforce must fail closed rather than be silently ignored.

See `12_v0.12_SANDBOX_RESOURCE_LIMITS.md`.

## Compatibility status

No versioned design or implementation row through v0.12 should be read as a promise that the current concrete interface will remain unchanged.

Until an explicit stable compatibility milestone is declared, breaking changes may modify or replace CLI commands, helper binaries, persisted state, provider interfaces, Base/image lifecycle, capability/policy schemas, client/agent integration, host bootstrap behavior, resource-budget design, and experimental runtime behavior.

Compatibility should not be preserved at the cost of an unsafe authority boundary, ambiguous ownership, silent data loss, or unnecessary architectural complexity. Material breaking changes should still be explicit, tested, and documented.
