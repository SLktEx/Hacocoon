# Rebaseline / Refactor Notes

Status: migration notes for the 2026-08-29 architecture rebaseline.

## Why this rebaseline exists

The earlier roadmap put local storage management, Git/worktree behavior, security, IDE integration, web interaction, and cloud runtime into a sequence that made v0.1 too large. Implementation began accumulating mechanisms before the smallest useful Hacocoon runtime had been proven end-to-end.

The new baseline narrows the product:

> Hacocoon receives a Workspace and executes it safely inside an isolated Environment.

AI orchestration and Git workflow ownership move above Hacocoon. Security capabilities remain inside Hacocoon because they protect the host/runtime boundary.

## Migration map

| Historical responsibility | New home |
|---|---|
| Session-oriented local runtime | Environment / Execution; refactor during v0.1 |
| Git repository/worktree ownership | WorkspaceProvider; optional from v0.2 |
| Btrfs / loop image / QCOW2 lifecycle | Storage or Environment adapter; not a v0.1 gate |
| VS Code / code-server / GUI | Client adapters, v0.3 |
| Authorization framework | Policy + Capability, v0.4 |
| Git push / GitHub operations | GitHub Capability, v0.5 |
| Agent CLI integration | Generic execution first; integration recipes/MCP in v0.6 |
| Model routing / task DAG / budgets | external orchestrator such as Daintree/Rookery |
| AWS and EC2/EBS | external capability + EnvironmentProvider, v0.7 |

## Existing code policy

Do not perform a destructive rewrite only to make the tree look new.

For each existing subsystem:

1. Keep it if it directly supports the new current gate.
2. Move it behind the correct boundary if it is useful later.
3. Stop wiring it into current behavior if it expands the current release scope.
4. Delete it only when it is clearly obsolete and tests prove there is no retained value.

The new v0.1 acceptance gate, not historical feature count, determines completion.

## Naming

Preferred current terms are `Workspace`, `WorkspaceLease`, `Environment`, `Execution`, `CapabilityRequest`, `PolicyDecision`, and `ApprovalRequest`.

Historical `Session` types may remain temporarily while code is migrated, but new architecture documentation and new public APIs should prefer `Environment`/`Workspace` terminology where it is semantically correct.
