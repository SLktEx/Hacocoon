# CODEX START HERE

Hacocoon has been rebaselined around one idea:

> **Hacocoon is a secure workspace runtime, not an AI orchestrator.**

The repository may contain working code from an older and broader v0.1 design. Treat that code as inventory, not as the current product boundary.

## Read first

1. `docs/00_REBASELINE_AND_ROADMAP.md`
2. `docs/00C_TERMINOLOGY_AND_BOUNDARIES.md`
3. `docs/00B_SECURITY_ARCHITECTURE.md`
4. `docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md`
5. `docs/IMPLEMENTATION_STATUS.md`
6. `docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`

`docs/00A_PLUGIN_ARCHITECTURE.md` is a later extension reference. Do not build a plugin framework during v0.1.

## Authoritative order

```text
0.1 Secure Workspace Runtime MVP
0.2 Workspace Abstraction & Lease
0.3 Client & Interactive Access
0.4 Policy & Capability Foundation
0.5 Git / GitHub Capability
0.6 Agent & Orchestrator Integration
0.7 Remote / Cloud Runtime & External Capabilities
```

## v0.1 job

Make this path work reliably:

```text
external directory
  -> create Incus environment
  -> mount directory read/write
  -> execute command
  -> open interactive shell
  -> delete environment and clean metadata
```

Target CLI:

```text
haco create --workspace ./repo dev
haco exec dev -- go test ./...
haco shell dev
haco delete dev
```

## Hard rules

- Do not implement v0.2-v0.7 while finishing v0.1.
- Do not make Git worktree a Core concept.
- Do not add GitHub, AWS, MCP, Web UI, agent routing, model budgets, notifications, or remote runtime to v0.1.
- Do not retain storage complexity in v0.1 merely because historical code already implements it.
- Do not create abstractions solely because a future provider might exist.
- Keep Incus/process/OS side effects behind a narrow adapter.
- Do not mount host HOME, `~/.ssh`, `~/.aws`, GitHub tokens, the Incus socket, or Manager state into the environment as a shortcut.
- A command executed inside the environment is untrusted with respect to host authority.
- Prefer standard CLI/protocol boundaries over Hacocoon-specific wrappers.

## Work method

1. Inventory current code and tests against `docs/IMPLEMENTATION_STATUS.md`.
2. Preserve useful code, but move/defer behavior that is outside the new v0.1 gate.
3. Implement one vertical slice at a time: domain -> Incus adapter -> create -> exec -> shell -> delete.
4. Add unit tests for deterministic logic and integration tests for the real Incus path.
5. Stop when the v0.1 acceptance gate passes.

When uncertain, choose the smallest design that preserves the trust boundary and keeps Workspace, Client, Orchestrator, and Environment responsibilities separate.
