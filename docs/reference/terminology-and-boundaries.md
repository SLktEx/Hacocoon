# Terminology and Boundaries

Status: authoritative terminology.

See [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md) for the design constraints attached to these boundaries and [`../design/trusted-host.md`](../design/trusted-host.md) for the current `haco-host` implementation slice.

## Physical Host

The actual Linux or WSL operating-system instance that runs the Hacocoon process and local platform primitives such as the Incus daemon, loop devices, Hacocoon-managed filesystem mounts, and other backend/bootstrap operations that inherently require host authority.

When documentation means this substrate specifically, use **Physical Host** rather than the ambiguous bare word "Host".

## `haco-host`

The Hacocoon-managed persistent trusted logical Host. On the local Incus backend it is the infrastructure instance literally named `haco-host`.

`haco-host` is not an Environment and is not an agent isolation boundary. It belongs to the trusted computing base. It is intended to become the normal host-like place for operator workflows, developer tooling, selected external-service operations, and optional platform integration while Physical Host primitives remain behind the Hacocoon boundary.

The current implementation provides lifecycle reconciliation and interactive entry but does not yet complete the planned Git/OCI/credential/Windows-interop or controller-channel migration.

## Workspace

User-selected files made available to an Environment. A Workspace may happen to be a Git repository or worktree, but Core treats it as opaque data plus access metadata.

A read-write Workspace is working data and may be modified or deleted by code running inside the Environment. The Workspace boundary limits what host data is intentionally exposed; it does not imply that writable Workspace contents are protected from the agent.

A Workspace's physical location is not part of Core semantics. Local deployments may choose `haco-host`, the Physical Host, or another provider-owned location as that architecture evolves.

## WorkspaceLease

A lifecycle-bound association between one Workspace and one Environment, including access mode and ownership information. It is introduced formally in v0.2. A lease is not required to imply wall-clock expiration.

## Environment

An isolated place in which commands run. The definition does not require a specific isolation technology or security strength.

Incus system containers are the first Environment implementation. Other implementations may use Incus VMs, microVMs, Kubernetes/scheduler-backed workloads, remote hosts, or another backend while preserving the same conceptual Environment boundary.

An Environment may grant strong local privileges, including `root`, without granting host authority. The selected backend is responsible for the isolation guarantees it claims to provide.

An Environment is distinct from trusted infrastructure such as `haco-host`. On the Incus backend the Environment name `host` is reserved because the provider-local runtime name would collide with the infrastructure instance `haco-host`.

## Environment backend

The concrete mechanism that realizes an Environment and its isolation boundary.

Backend-specific properties include shared-kernel versus separate-kernel isolation, lifecycle mechanics, storage/network attachment, supported resource controls, and other guarantees. These properties must not silently become universal Core assumptions.

## Execution

One command or interactive process executed inside an Environment, with explicit exit/error/result handling.

## Client

A human-facing or tool-facing entry point that asks Hacocoon to operate on a Workspace/Environment. Examples include the CLI, VS Code integration, shell scripts, and external orchestrators using a stable Hacocoon interface.

A Client does not need raw provider authority. In particular, future `haco` invocations from inside `haco-host` should target a Hacocoon-owned control channel rather than requiring the Incus socket in that instance.

## Orchestrator

A system that decides tasks, agents, retries, model selection, worktrees, budgets, and development workflow. Examples may include Daintree or Rookery. An Orchestrator is outside Hacocoon Core.

External orchestrators should integrate through a stable Hacocoon client/control surface. They should not need to know whether a particular trusted operation is internally executed on the Physical Host or in `haco-host`.

## WorkspaceProvider

A v0.2+ seam that produces or resolves a Workspace. The direct external-path behavior used by v0.1 does **not** require a formal provider interface. A Git worktree provider is optional convenience functionality, not Core semantics.

## EnvironmentProvider

The conceptual boundary for creating, inspecting, connecting to, executing inside, and destroying Environments. Incus is the first implementation. v0.1 may use a concrete Incus adapter without prematurely generalizing a provider framework; a formal provider interface should be justified by testing or another real implementation.

When generalized, the provider boundary should expose capabilities/guarantees rather than force Core to branch on backend names.

Trusted infrastructure instances such as the local Incus `haco-host` are provider/platform support resources and must not be silently modeled as ordinary Environments merely because the same backend creates them.

## CapabilityRequest

A request to perform an operation that crosses the untrusted execution boundary into privileged host or external-service authority. This vocabulary becomes an implementation concern in v0.4.

## PolicyDecision

The result of evaluating a CapabilityRequest: `allow`, `deny`, or `require-approval`. Introduced in v0.4.

## ApprovalRequest

A durable request for human authorization of a privileged action. Introduced in v0.4.

## Trusted computing base (TCB)

The components that must remain trusted for a selected backend's containment guarantee to hold.

For the Incus system-container backend this includes at least the Physical Host Linux kernel, Incus daemon/control plane, trusted Hacocoon Physical Host process, and the persistent `haco-host` instance when provisioned. A stronger backend may move or reduce parts of this trust boundary, but Core must not claim a stronger guarantee than the backend actually provides.

## Historical Session terminology

Existing code still contains `Session` while the rebaseline is implemented. `Session` is an implementation-migration term, not the preferred new architecture vocabulary. Do not create new public architecture coupling around it; migrate toward Workspace + Environment + Execution where that distinction improves clarity.
