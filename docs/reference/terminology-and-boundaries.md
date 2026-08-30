# Terminology and Boundaries

Status: authoritative terminology.

See [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md) for the design constraints attached to these boundaries.

## Workspace

User-selected files made available to an Environment. A Workspace may happen to be a Git repository or worktree, but Core treats it as opaque data plus access metadata.

A read-write Workspace is working data and may be modified or deleted by code running inside the Environment. The Workspace boundary limits what host data is intentionally exposed; it does not imply that writable Workspace contents are protected from the agent.

## WorkspaceLease

A lifecycle-bound association between one Workspace and one Environment, including access mode and ownership information. It is introduced formally in v0.2. A lease is not required to imply wall-clock expiration.

## Environment

An isolated place in which commands run. The definition does not require a specific isolation technology or security strength.

Incus system containers are the first Environment implementation. Other implementations may use Incus VMs, microVMs, Kubernetes/scheduler-backed workloads, remote hosts, or another backend while preserving the same conceptual Environment boundary.

An Environment may grant strong local privileges, including `root`, without granting host authority. The selected backend is responsible for the isolation guarantees it claims to provide.

## Environment backend

The concrete mechanism that realizes an Environment and its isolation boundary.

Backend-specific properties include shared-kernel versus separate-kernel isolation, lifecycle mechanics, storage/network attachment, supported resource controls, and other guarantees. These properties must not silently become universal Core assumptions.

## Execution

One command or interactive process executed inside an Environment, with explicit exit/error/result handling.

## Client

A human-facing or tool-facing entry point that asks Hacocoon to operate on a Workspace/Environment. Examples include the CLI, VS Code integration, shell scripts, and external orchestrators using a stable Hacocoon interface.

## Orchestrator

A system that decides tasks, agents, retries, model selection, worktrees, budgets, and development workflow. Examples may include Daintree or Rookery. An Orchestrator is outside Hacocoon Core.

## WorkspaceProvider

A v0.2+ seam that produces or resolves a Workspace. The direct external-path behavior used by v0.1 does **not** require a formal provider interface. A Git worktree provider is optional convenience functionality, not Core semantics.

## EnvironmentProvider

The conceptual boundary for creating, inspecting, connecting to, executing inside, and destroying Environments. Incus is the first implementation. v0.1 may use a concrete Incus adapter without prematurely generalizing a provider framework; a formal provider interface should be justified by testing or another real implementation.

When generalized, the provider boundary should expose capabilities/guarantees rather than force Core to branch on backend names.

## CapabilityRequest

A request to perform an operation that crosses the untrusted execution boundary into privileged host or external-service authority. This vocabulary becomes an implementation concern in v0.4.

## PolicyDecision

The result of evaluating a CapabilityRequest: `allow`, `deny`, or `require-approval`. Introduced in v0.4.

## ApprovalRequest

A durable request for human authorization of a privileged action. Introduced in v0.4.

## Trusted computing base (TCB)

The components that must remain trusted for a selected backend's containment guarantee to hold.

For the Incus system-container backend this includes at least the host Linux kernel, Incus daemon/control plane, and trusted Hacocoon host process. A stronger backend may move or reduce parts of this trust boundary, but Core must not claim a stronger guarantee than the backend actually provides.

## Historical Session terminology

Existing code still contains `Session` while the rebaseline is implemented. `Session` is an implementation-migration term, not the preferred new architecture vocabulary. Do not create new public architecture coupling around it; migrate toward Workspace + Environment + Execution where that distinction improves clarity.
