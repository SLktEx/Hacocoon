# v0.6 — Agent & Orchestrator Integration

Status: **roadmap contract implemented on `main`.** Generic `haco run`, machine-readable output, and security-event export exist; orchestration remains outside Hacocoon.

## Goal

Make Hacocoon easy to use below AI development orchestration systems without making Hacocoon itself an orchestrator.

## Generic execution first

Agent CLIs are just commands from the runtime's perspective:

```text
haco run --workspace <path> -- codex
haco run --workspace <path> -- claude
```

`run` remains a generic execution convenience over the Environment lifecycle, not a new Core task/orchestration model.

## In scope

- `haco run` or equivalent short-lived execution UX.
- Structured execution/status result for machine clients.
- Stable conceptual events/status surface.
- Agent wrapper examples.
- External orchestrator integration recipes.
- MCP adapter if it proves useful for interoperability.
- Export of security-approval events to an external client.

## External orchestrator responsibility

External orchestrators may own:

- task decomposition;
- model/agent choice;
- retries;
- model budget;
- worktree creation;
- development review queues;
- merge workflow.

Hacocoon owns the secure execution boundary below them.

## Two Human-in-the-loop layers

```text
Development approval -> Orchestrator / GitHub / Human
Security approval    -> Hacocoon policy/capability boundary
```

Do not merge these responsibilities into one giant workflow engine.

## Compatibility note

Machine-facing JSON/event formats are still pre-1.0. They may evolve, but a format change must not silently move orchestration responsibility into Hacocoon or weaken the security-approval boundary.
