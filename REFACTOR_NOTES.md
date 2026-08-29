# Rebaseline / Refactor Notes

Status: historical migration notes for the 2026-08-29 architecture rebaseline. The initial implementation progression has since reached v0.7; use `docs/IMPLEMENTATION_STATUS.md` for current repository reality.

## Why this rebaseline exists

The earlier roadmap put local storage management, Git/worktree behavior, security, IDE integration, web interaction, and cloud runtime into a sequence that made the first useful Hacocoon slice too large. Implementation began accumulating mechanisms before the smallest useful secure workspace runtime had been proven end-to-end.

The rebaseline narrowed the product to:

> Hacocoon receives a Workspace and executes it safely inside an isolated Environment.

AI orchestration and Git workflow ownership stay above Hacocoon. Security capabilities remain inside Hacocoon because they protect the host/runtime and external-authority boundaries.

## Migration map

| Historical responsibility | Rebaselined home | Current note |
|---|---|---|
| Session-oriented local runtime | Environment / Execution | public runtime path uses the newer model; historical code may remain as inventory |
| Git repository/worktree ownership | Workspace boundary / optional provider | Core treats Workspace as opaque |
| Btrfs / loop image / QCOW2 lifecycle | Storage or Environment adapter | historical/provider detail, not a Core requirement |
| VS Code / code-server / GUI | Client boundary | client access implemented without IDE ownership |
| Authorization framework | Policy + Capability | implemented with fail-closed policy/approval/audit |
| Git push / GitHub operations | Git/GitHub capability | brokered host-side push implemented |
| Agent CLI integration | Generic execution | `haco run` and machine/event surfaces implemented; orchestration remains external |
| Model routing / task DAG / budgets | external orchestrator | explicitly outside Hacocoon |
| AWS and EC2/EBS | capability + runtime/storage adapters | v0.7 implementation exists; EC2 remains experimental/default-off |

## Existing code policy

Do not perform a destructive rewrite only to make the tree look new.

For each subsystem:

1. Keep it when it supports a current architecture contract.
2. Move it behind the correct boundary when the responsibility is valid but misplaced.
3. Stop wiring historical behavior into public paths when it expands or contradicts the current architecture.
4. Delete obsolete code when tests establish there is no retained value.
5. Do not preserve accidental compatibility when it forces unsafe or confusing ownership.

The architecture baseline and current code reality—not historical feature count—determine what should remain.

## Naming

Preferred current terms are `Workspace`, `WorkspaceLease`, `Environment`, `Execution`, `CapabilityRequest`, `PolicyDecision`, and `ApprovalRequest`.

Historical `Session` types may remain only where migration or compatibility work still justifies them. New architecture documentation and public APIs should use the current vocabulary when semantically correct.

## Pre-1.0 refactoring rule

Hacocoon is still pre-1.0. Refactoring may intentionally break CLI/API/state/config behavior when doing so produces a clearer or safer system.

That freedom should be used to remove accidental complexity, not to create churn. Breaking refactors must still protect recoverable data, surface migration impact, update tests, and keep provider/authority boundaries explicit.
