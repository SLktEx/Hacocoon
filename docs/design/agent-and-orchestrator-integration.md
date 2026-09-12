# Agent and orchestrator integration

Status: **implemented execution boundary; orchestration remains external**.

Hacocoon supplies Workspace, Environment, Execution and Policy/Capability boundaries
under a client or orchestrator. It does not own task graphs, model selection, token
budgets, worktree orchestration, retries, code review or merge decisions.

For a noninteractive command, use [temporary execution](temporary-execution.md):

```bash
haco run --workspace managed:review --read-only --json -- git status --short
```

The Workspace must be available for a new lease. A retained stopped Env still holds
its lease. Agent tools must already exist in the selected Base or be prepared in a
retained Env, and must support noninteractive input; an interactive agent shell
is not implied by `haco run`. Use [ordinary SSH](client-and-interactive-access.md)
for retained interactive work.

The caller chooses independent Workspaces, invokes exact argv, consumes execution/
truncation/cleanup results, then decides its own retry or review. It must not retry
an unknown external outcome as though nothing happened. Runtime-owned cleanup
and leases stay inside the canonical lifecycle.

Development review belongs to the caller/GitHub/human. Security approval belongs
to Hacocoon Policy and the trusted capability boundary. Notification delivery,
agent output and task completion never grant authority.

Clients observe [minimized interaction events](../reference/interaction-events.md)
without capability parameters or credentials. Historical raw audit export is
documented only in [legacy CLI migration](../reference/cli-migration.md#legacy-event-cursor).
Client APIs and pre-1.0 wire formats may change; use the
[client adapter contract](../reference/client-adapter.md).
An MCP adapter is a possible optional integration, not an implemented Core dependency.
