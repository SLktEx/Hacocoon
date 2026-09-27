# Agent and orchestrator integration

Status: **implemented execution boundary; orchestration remains external**.

Hacocoon supplies Workspace, Environment, Execution and Policy/Capability boundaries
under a client or orchestrator. It does not own task graphs, model selection, token
budgets, worktree orchestration, retries, code review or merge decisions.

Create independent work with `haco open --new [IMAGE]`, or use `haco create IMAGE`
for a stopped Environment. Use [ordinary SSH](client-and-interactive-access.md)
for commands and interactive work. The caller owns orchestration, retry and review;
Hacocoon owns lifecycle, leases and permission checks. See [creation](environment-creation.md).

Development review belongs to the caller/GitHub/human. Security approval belongs
to Hacocoon Policy and the trusted capability boundary. Notification delivery,
agent output and task completion never grant authority.

Clients observe [minimized interaction events](../reference/interaction-events.md)
without capability parameters or credentials. Client APIs and pre-1.0 wire formats may change; use the
[client adapter contract](../reference/client-adapter.md).
An MCP adapter is a possible optional integration, not an implemented Core dependency.
