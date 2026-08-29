# Terminology and Boundaries

Status: authoritative terminology.

## Workspace

User-selected files made available to an Environment. A Workspace may happen to be a Git repository or worktree, but Core treats it as opaque data plus access metadata.

## WorkspaceLease

A time-bounded association between one Workspace and one Environment, including access mode and ownership information. Introduced formally in v0.2.

## Environment

An isolated place in which commands run. Incus system containers are the first Environment implementation. EC2 or other runtimes may be added later behind the same conceptual boundary.

## Execution

One command or interactive process executed inside an Environment, with explicit exit/error/result handling.

## Client

A human-facing or tool-facing entry point that asks Hacocoon to operate on a Workspace/Environment. Examples: CLI, VS Code integration, shell scripts, future Web UI.

## Orchestrator

A system that decides tasks, agents, retries, model selection, worktrees, budgets, and development workflow. Examples may include Daintree or Rookery. An Orchestrator is outside Hacocoon Core.

## WorkspaceProvider

Produces or resolves a Workspace. `ExternalPathWorkspace` is the initial behavior. A Git worktree provider is optional convenience functionality, not Core semantics.

## EnvironmentProvider

Creates, inspects, and destroys Environments. Incus is first; cloud runtimes are later.

## CapabilityRequest

A request to perform an operation that crosses the untrusted execution boundary into privileged host or external-service authority.

## PolicyDecision

The result of evaluating a CapabilityRequest: `allow`, `deny`, or `require-approval`.

## ApprovalRequest

A durable request for human authorization of a privileged action.

## Historical Session terminology

Existing code may still contain `Session` while the rebaseline is implemented. Do not create new architecture coupling around the old term. Migrate toward Workspace + Environment + Execution where that distinction improves clarity.
