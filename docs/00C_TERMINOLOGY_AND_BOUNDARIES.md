# Terminology and Boundaries

Status: authoritative terminology.

## Workspace

User-selected files made available to an Environment. A Workspace may happen to be a Git repository or worktree, but Core treats it as opaque data plus access metadata.

## WorkspaceLease

A lifecycle-bound association between one Workspace and one Environment, including access mode and ownership information. It is introduced formally in v0.2. A lease is not required to imply wall-clock expiration.

## Environment

An isolated place in which commands run. Incus system containers are the first Environment implementation. EC2 or other runtimes may be added later behind the same conceptual boundary.

## Execution

One command or interactive process executed inside an Environment, with explicit exit/error/result handling.

## Client

A human-facing or tool-facing entry point that asks Hacocoon to operate on a Workspace/Environment. Examples include the CLI, VS Code integration, shell scripts, and external orchestrators using a stable Hacocoon interface.

## Orchestrator

A system that decides tasks, agents, retries, model selection, worktrees, budgets, and development workflow. Examples may include Daintree or Rookery. An Orchestrator is outside Hacocoon Core.

## WorkspaceProvider

A v0.2+ seam that produces or resolves a Workspace. The direct external-path behavior used by v0.1 does **not** require a formal provider interface. A Git worktree provider is optional convenience functionality, not Core semantics.

## EnvironmentProvider

The conceptual boundary for creating, inspecting, and destroying Environments. Incus is the first implementation. v0.1 may use a concrete Incus adapter without prematurely generalizing a provider framework; a formal provider interface should be justified by testing or another implementation.

## CapabilityRequest

A request to perform an operation that crosses the untrusted execution boundary into privileged host or external-service authority. This vocabulary becomes an implementation concern in v0.4.

## PolicyDecision

The result of evaluating a CapabilityRequest: `allow`, `deny`, or `require-approval`. Introduced in v0.4.

## ApprovalRequest

A durable request for human authorization of a privileged action. Introduced in v0.4.

## Historical Session terminology

Existing code still contains `Session` while the rebaseline is implemented. `Session` is an implementation-migration term, not the preferred new architecture vocabulary. Do not create new public architecture coupling around it; migrate toward Workspace + Environment + Execution where that distinction improves clarity.
