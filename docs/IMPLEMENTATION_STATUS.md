# Implementation Status

Status date: 2026-08-29, after the v0.8 Client Adapters & VS Code Integration implementation pass.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Release specifications under `docs/01_...` through `docs/08_...` remain the versioned design references for their roadmap stages.

Hacocoon is still **pre-1.0**. An area being implemented on `main` means the code path exists; it does not mean its CLI/API/state/config surface is frozen, production support is guaranteed, or every real-provider/client acceptance test has passed.

The repository still contains historical code from the pre-rebaseline roadmap. Existing historical packages do not automatically define the current public architecture.

| Area | Current repository reality | Release | Validation status |
|---|---|---:|---|
| Secure Workspace Runtime | public Environment path supports `haco create --workspace`, `haco exec`, `haco shell`, and `haco delete` | v0.1 | unit and process-boundary integration pass; supported-host real Incus acceptance remains pending |
| Workspace model | `Workspace`, `Environment`, `ExecutionResult`, canonical external-path Workspace identity, and persisted Workspace leases are implemented | v0.1-v0.2 | unit, persistence, concurrency, and process-boundary tests pass |
| Workspace lease safety | RO/RW leases, RW conflict prevention, stale-lease recovery, and process serialization are implemented | v0.2 | unit/concurrency/integration tests pass |
| Incus Environment provider | concrete local Incus Environment implementation remains the default runtime | v0.1+ | unit/process tests pass; real Incus host acceptance remains pending in this sandbox |
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
| Client adapter boundary | IDE-specific launch/configuration remains outside Core; Core does not depend on VS Code, Daintree, JetBrains, or client-native configuration | v0.8 | architecture/documentation contract plus separate adapter binary |
| Btrfs / raw / QCOW2 historical storage | historical local storage implementation remains in the repository | historical / provider detail | not part of the current Core Environment model and not a compatibility commitment |
| CI | Go version matrix tests, `go vet`, race detector, docs consistency, and existing non-host-dependent E2Es are enabled | cross-cutting | latest v0.8 PR CI must pass before merge; real-provider/client acceptance remains separate |

## Current implementation state

The implemented progression now reaches v0.8:

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
```

The v0.8 adapter deliberately does not add AI chat, model selection, task planning, worktree orchestration, or IDE-specific concepts to Core. The intended interactive development path is that VS Code (including its own AI/coding-agent UI) connects through standard Remote-SSH and operates inside the Hacocoon Environment.

The coding agent may be intentionally permissive inside the isolated Environment. Authority outside the Environment remains mediated by the existing Hacocoon Policy/Capability/Audit boundary.

The v0.7 EC2 provider remains **experimental and disabled by default**. Shipping the implementation does not make EC2 a normal supported backend. Real AWS/EC2/SSM/EBS acceptance has not been performed from the current sandbox and must not be reported as passed.

Likewise, the real Incus and VS Code Remote-SSH acceptance paths require suitable hosts. Unit tests, process-boundary integrations, fake-provider E2Es, race checks, vet, build, and repository CI are not substitutes for those host/client acceptance runs.

## v0.8 client workflow

The current convenience path is:

```bash
haco-vscode open .
```

Conceptually this performs:

```text
local Workspace
  -> create/reuse Hacocoon Environment
  -> prepare loopback-only SSH
  -> create adapter-owned SSH host entry
  -> code --remote ssh-remote+<alias> /workspace
```

Cleanup is:

```bash
haco-vscode delete .
```

When the adapter runs in WSL for a Windows desktop VS Code client, the SSH configuration must be managed in the Windows client profile, not only under the WSL user's Linux home.

## Compatibility status

No v0.1-v0.8 implementation row should be read as a promise that the current concrete interface will remain unchanged.

Until an explicit stable compatibility milestone is declared, breaking changes may modify or replace:

- CLI commands, helper binaries, flags, and output;
- persisted state and migrations;
- provider interfaces and configuration;
- capability/policy schemas;
- client-adapter configuration;
- experimental runtime behavior.

Compatibility should not be preserved at the cost of an unsafe authority boundary, ambiguous ownership, silent data loss, or unnecessary architectural complexity. Material breaking changes should still be explicit, tested, and documented.

Do not infer release/tag readiness solely from this implementation status. Tagging decisions must also respect the acceptance requirements and the intended stability level of the corresponding release.
