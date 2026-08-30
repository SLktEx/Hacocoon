# Adapter and Extension Architecture

Status: cross-cutting architecture reference. This is **not** a v0.1 implementation requirement.

See also [`DESIGN_PRINCIPLES.md`](DESIGN_PRINCIPLES.md) for the product-level constraints this architecture must preserve.

## Goal

Hacocoon keeps a small Core while concrete environment, workspace, capability, approval, storage, client, and developer-tool integrations evolve independently.

Use ordinary Go package boundaries and ports/adapters first. A dynamic plugin framework is added only if deployment or third-party extension needs justify it. The CLI `haco plugin ...` namespace is the user-facing boundary for optional integrations; it does not imply that every plugin must be a dynamically loaded shared object.

## Core boundary

Core owns only the behavior needed to create, isolate, connect to, execute inside, and tear down Hacocoon Environments, plus the generic policy/approval/event boundaries required to do that safely.

Core must not require a particular developer workload or toolchain inside an Environment.

In particular, Core must not require or assume:

- containerd;
- nerdctl;
- Docker CLI or Docker Engine;
- an OCI registry;
- OCI image telemetry/Seed promotion;
- Git or GitHub;
- cloud-provider CLIs;
- VS Code or another IDE.

An Environment may contain any of those tools because a Base/Seed, operator, or optional plugin chose to provide them.

## Environment backends and isolation strength

Hacocoon is not defined by Incus. Incus system containers are the first concrete Environment backend, not the permanent definition of an Environment.

A future backend may use a container, VM, microVM, scheduler, remote host, or another isolation mechanism. Core should depend on Environment lifecycle and capabilities rather than backend names.

Different backends may provide different isolation guarantees. A lightweight container backend may share the host kernel, while a VM or microVM backend may provide a separate kernel. This difference is a backend property, not a reason to fork Core semantics.

When a requested guarantee cannot be provided by the selected backend, fail explicitly rather than pretending all backends are equivalent. Do not spread `if backend == ...` conditionals through Core; introduce capability-oriented seams once multiple real implementations justify them.

Environment-local privilege is also backend-scoped. An agent may be `root` inside an Environment without receiving host authority, provided the backend preserves the boundary and does not expose host credentials, control sockets, or unrelated host filesystem paths.

## Candidate seams

These are long-term conceptual seams, **not a checklist of interfaces to create now**:

```text
WorkspaceProvider
EnvironmentProvider
CommandExecutor
CapabilityProvider
PolicyEvaluator
ApprovalProvider
EventSink
Plugin
```

Promote a seam into a Go interface when a second implementation, a stable test boundary, or a real replacement requirement makes it useful.

Core domain values must not import Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, OCI/container tooling, storage-backend, or cloud-provider implementation packages.

## Optional OCI plugin

Container tooling is an optional developer-workload integration implemented under `modules/plugin/oci`.

The OCI plugin may provide profiles backed by `nerdctl` or the genuine Docker CLI, OCI usage telemetry, Seed recommendations, local-registry helpers, and Docker Engine compatibility packaging. Those are plugin responsibilities even when a project-maintained development profile enables them for convenience.

The plugin is opt-in at host composition time:

```text
HACO_PLUGIN_OCI=nerdctl   # use nerdctl for OCI inventory
HACO_PLUGIN_OCI=docker    # use Docker CLI for OCI inventory
unset HACO_PLUGIN_OCI     # no OCI plugin; Core still works
```

The absence of the plugin must not make `haco create`, `haco run`, `haco exec`, connection management, policy, approvals, or Environment lifecycle unavailable merely because `nerdctl`, Docker, or containerd is missing.

Plugin-owned CLI functionality lives under:

```text
haco plugin oci ...
```

Core Base inspection remains under `haco base ...` because that command describes Hacocoon Environment Bases, not OCI workload images.

## Release placement

```text
v0.1
  direct external workspace path
  concrete Incus adapter

v0.2+
  WorkspaceProvider boundary when useful
  ExternalPath provider
  optional GitWorktree provider

v0.4+
  Policy / Approval / Capability seams

v0.5
  GitHub capability adapter

v0.7
  provider-neutral remote/cloud routing seam
  concrete EC2/AWS/EBS implementation is currently deferred

v0.15
  OCI usage telemetry and Seed recommendation

v0.16
  OCI image deletion

v0.17
  optional Docker compatibility plugin foundation

v0.18+
  optional Local OCI Registry and later Seed/COW work remain outside Core
```

The first Incus implementation does not by itself require a generalized `EnvironmentProvider` framework. The second real environment backend is the natural point to validate that seam.

The provider-neutral v0.7 seam remains part of the architecture, but the concrete EC2/AWS/EBS implementation is intentionally absent from the active tree until the local/provider contracts are stable enough for meaningful cloud acceptance.

## Do not over-generalize v0.1

v0.1 should not create interfaces merely because the roadmap names future providers. Start with the smallest concrete Incus/external-path vertical slice.

Advanced storage or OCI code present in the repository is optional implementation inventory, not proof that storage or container tooling belongs in Core or in the v0.1 architecture.

## Workspace ownership rule

A workspace is opaque to the runtime. If Daintree, Rookery, VS Code, a shell script, or a human created the directory/worktree, Hacocoon accepts the path without taking ownership of the upstream Git workflow.

An optional Git-worktree WorkspaceProvider, when implemented, is convenience functionality for standalone Hacocoon use—not the definition of a Workspace.

## External orchestrators

Orchestrators are clients above Hacocoon, not plugins inside Core. They may call the CLI, a future MCP adapter, or another stable protocol. Hacocoon never depends on their task DAG, model selection, retries, or budget model.
