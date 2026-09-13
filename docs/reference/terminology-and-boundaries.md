# Terminology and boundaries

Status: authoritative terminology.

See [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md) for the design constraints attached to these boundaries and [`../design/trusted-host.md`](../design/trusted-host.md) for the current `haco-host` implementation slice.

## Physical Host

The actual Linux or WSL operating-system instance that runs the Hacocoon process and local platform primitives such as the Incus daemon, loop devices, Incus-owned filesystem mounts, and other backend/bootstrap operations that inherently require host authority.

When documentation means this substrate specifically, use **Physical Host** rather than the ambiguous bare word "Host".

## `haco-host`

The Hacocoon-managed persistent trusted logical Host. On the local Incus backend it is the infrastructure instance literally named `haco-host`.

`haco-host` is not an Environment and is not an agent isolation boundary. It belongs to the trusted computing base. It is the normal management place for operator workflows, developer tooling, selected external-service operations, and optional platform integration while Physical Host primitives remain behind the Hacocoon boundary.

Current setup, Git and optional OCI operations, Windows interop and narrow controller transport use this boundary. Exact remaining limits belong to [implementation status](../IMPLEMENTATION_STATUS.md).

## Workspace

User-selected files made available to an Environment. A Workspace may happen to be a Git repository or worktree, but Core treats it as opaque data plus access metadata.

A read-write Workspace is working data and may be modified or deleted by code running inside the Environment. The Workspace boundary limits what host data is intentionally exposed; it does not imply that writable Workspace contents are protected from the agent.

A Workspace's physical location is not part of Core semantics. Local deployments may choose `haco-host`, the Physical Host, or another provider-owned location as that architecture evolves.

## WorkspaceLease

A lifecycle-bound association between one Workspace and one Environment, including access mode and ownership information. A lease is not required to imply wall-clock expiration.

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

A Client does not need raw provider authority. In particular, product `haco` invocations from inside `haco-host` target a Hacocoon-owned control channel rather than requiring the Incus socket in that instance.

## Orchestrator

A system that decides tasks, agents, retries, model selection, worktrees, budgets, and development workflow. Examples may include Daintree or Rookery. An Orchestrator is outside Hacocoon Core.

External orchestrators should integrate through a stable Hacocoon client/control surface. They should not need to know whether a particular trusted operation is internally executed on the Physical Host or in `haco-host`.

## WorkspaceProvider

The seam that produces or resolves external or managed Workspaces. Managed Git preparation is an integration; Git worktree orchestration is not Core semantics.

## EnvironmentProvider

The conceptual boundary for creating, inspecting, connecting to, executing inside, and destroying Environments. Incus is the first implementation. The current provider/router interfaces separate native operations from Core; additional abstractions need testing or a real implementation to justify them.

The provider boundary exposes capabilities/guarantees rather than force Core to branch on backend names.

Trusted infrastructure instances such as the local Incus `haco-host` are provider/platform support resources and must not be silently modeled as ordinary Environments merely because the same backend creates them.

## CapabilityRequest

A request to perform an operation that crosses the untrusted execution boundary into privileged host or external-service authority.

## PolicyDecision

The result of evaluating a CapabilityRequest: `allow`, `deny`, or `require-approval`.

## ApprovalRequest

A request for human authorization of a privileged action. Pending session lifetime and durable decision/audit receipts are distinct; unfinished requests are not automatically replayed after controller restart.

## Trusted computing base (TCB)

The components that must remain trusted for a selected backend's containment guarantee to hold.

For the Incus system-container backend this includes at least the Physical Host Linux kernel, Incus daemon/control plane, trusted Hacocoon Physical Host process, and the persistent `haco-host` instance when provisioned. A stronger backend may move or reduce parts of this trust boundary, but Core must not claim a stronger guarantee than the backend actually provides.

## Historical Session terminology

Existing code still contains `Session` while the rebaseline is implemented. `Session` is an implementation-migration term, not the preferred new architecture vocabulary. Do not create new public architecture coupling around it; migrate toward Workspace + Environment + Execution where that distinction improves clarity.

## Base

A named starting point resolved once to an immutable `BaseRef` for Environment creation.
It selects guest contents, not authority. Saved rootfs is independent of the original
Base filesystem/image; the saved Base reference is provenance.

## OCI Store

An optional, independently owned persistent runtime-data resource, reserved with its
Workspace by canonical Environment lifecycle. Stop retains its lease; Env deletion
releases attachment after positive absence and retains Store data. Processes/sockets
are not persistent data. Source-only Host areas cannot be attached as workloads.

## Core, Standard and Plugin

Core defines stable product contracts and trust boundaries. Standard supplies
maintained, replaceable defaults such as Incus and egress enforcement. Plugins add
optional integrations; their absence must leave a useful Core. See
[plugin architecture](../design/plugin-architecture.md).

## Env, Base and OCI Store

**Env** is the CLI shorthand for **Environment**, the isolated execution place.
A **Base** is an Environment starting point, not the working files. A
**Workspace** holds working files independently of the Env runtime. An
**OCI Store** is optional retained container image/build/runtime data; its
lifetime is separate from Workspace and Env. `haco env stop` retains the Env;
`haco env delete` removes its runtime/rootfs while retaining Workspace, OCI
Store and independent snapshots. Explicit retained-data deletion is separate.
See [daily workflow](daily-workflow.md) for execution locations and commands.
