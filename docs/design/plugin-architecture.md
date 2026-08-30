# Adapter and Extension Architecture

Status: cross-cutting architecture reference. This is **not** a v0.1 implementation requirement.

See also [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md) for the product-level constraints this architecture must preserve.

## Goal

Hacocoon keeps a small Core while concrete environment, workspace, capability, approval, storage, client, and developer-tool integrations evolve independently.

Use ordinary Go package boundaries and ports/adapters first. A dynamic plugin framework is added only if deployment or third-party extension needs justify it. The CLI `haco plugin ...` namespace is the user-facing boundary for optional integrations; it does not imply that every plugin must be a dynamically loaded shared object.

## Core / Standard / Plugin classification

Hacocoon uses three conceptual ownership levels. Frequency of use alone does not decide whether something belongs in Core.

### Core

Core owns the stable product contracts and control logic that define Hacocoon itself:

- Environment lifecycle and execution semantics;
- generic Policy / Approval / Capability semantics;
- requests and controller contracts for crossing the Environment boundary;
- provider-facing contracts required to enforce those decisions;
- state and audit semantics that must remain consistent across implementations.

Core describes **what must be controlled**, not the backend-specific mechanism used to control it.

For example, outbound network access is a Hacocoon concern. The request model, policy decision (`allow`, `deny`, `require-approval`), approval binding, and controller/provider contract therefore belong to Core. HTTP CONNECT, SOCKS, nftables, Incus ACL manipulation, Kubernetes NetworkPolicy, or another concrete enforcement mechanism do not.

### Standard

Standard components are project-maintained default implementations that most normal Hacocoon installations are expected to use, but which are still replaceable implementations rather than Core semantics.

Examples include:

- the current Incus Environment backend;
- the project-maintained default egress enforcement/proxy implementation once implemented;
- default notification or interaction adapters shipped for normal installations;
- other maintained adapters needed to make the default Hacocoon distribution useful out of the box.

A Standard component may be enabled by default and distributed with Hacocoon without becoming a permanent Core dependency. Another implementation must be able to satisfy the same Core contract later.

### Plugin

Plugins are optional workload integrations, alternative implementations, or specialized extensions whose absence still leaves a generally useful Hacocoon installation.

Examples include:

- OCI/container tooling based on nerdctl, Docker, or containerd;
- alternative egress providers such as a specialized enterprise proxy;
- optional Git convenience surfaces;
- IDE-specific integrations beyond the default distribution;
- workload-specific or organization-specific tooling.

The practical classification test is:

> Would a normal Hacocoon distribution still be a complete and generally useful Hacocoon without this component?

If yes, it is usually Plugin territory. If users normally need one implementation but the implementation must remain replaceable, it is Standard. If changing or removing it would change Hacocoon's product semantics or security contract, the contract belongs in Core.

This means the default egress proxy/enforcer should be Standard rather than Plugin, while the egress request/policy/controller interfaces remain Core. nerdctl remains an optional Plugin even if a project-maintained development profile commonly enables it.

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

An Environment may contain any of those tools because a Base/Seed, operator, Standard component, or optional plugin chose to provide them.

## Environment backends and isolation strength

Hacocoon is not defined by Incus. Incus system containers are the first concrete Environment backend and the current Standard backend, not the permanent definition of an Environment.

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
EgressController
EgressProvider
EventSink
Plugin
```

Promote a seam into a Go interface when a second implementation, a stable test boundary, or a real replacement requirement makes it useful. Security-critical product contracts such as generic egress authorization may justify an explicit Core contract before multiple concrete providers exist when the contract itself is part of Hacocoon's security semantics.

Core domain values must not import Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, OCI/container tooling, storage-backend, proxy implementation, or cloud-provider implementation packages.

## Outbound network / egress boundary

Hacocoon's product semantics include controlling when an Environment may cross its external network boundary.

Conceptually:

```text
untrusted Environment
       |
       | EgressRequest(destination, protocol, port, metadata)
       v
Core Policy / Approval / EgressController
       |
       | allow / deny / require-approval
       v
EgressProvider contract
       |
       v
Standard or alternative enforcement implementation
```

Core must be able to express and authorize the request without knowing whether enforcement uses a proxy, firewall, gateway, runtime ACL, or scheduler-native network policy.

The maintained default enforcement implementation belongs to Standard because a normal Hacocoon installation is expected to have usable controlled egress. Alternative enforcement implementations may be Plugins or provider-specific adapters.

The v0.13 Incus managed network is currently only the default-deny substrate. Domain-aware allow/approval enforcement and the generic Core-to-provider egress control path are not implied to be implemented merely by this architecture classification.

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
  OCI Seed Builder and storage-driver COW lifecycle
  first repository slice implemented; real-host/COW acceptance pending

v0.18
  optional Docker compatibility plugin
  repository implementation complete; real-host acceptance pending

unversioned / deferred
  optional Local OCI Registry remains outside Core
```

The Core/Standard/Plugin classification is cross-cutting architecture and does not consume a milestone by itself. A future independently useful domain-aware egress feature should receive its own milestone when implementation work begins; this document does not mark that feature implemented.

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
