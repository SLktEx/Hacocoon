# Adapter and Extension Architecture

Status: cross-cutting architecture reference. This is **not** a v0.1 implementation requirement.

## Goal

Hacocoon keeps a small Core while concrete environment, workspace, capability, approval, storage, client, and developer-tool integrations evolve independently.

Use ordinary Go package boundaries and ports/adapters first. A dynamic plugin framework is added only if deployment or third-party extension needs justify it. The CLI `haco plugin ...` namespace is the user-facing boundary for optional integrations; it does not imply that every plugin must be a dynamically loaded shared object.

## Core boundary

Core owns only the behavior needed to create, isolate, connect to, execute inside, and tear down Hacocoon Environments, plus the generic policy/approval/event boundaries required to do that safely.

Core must not require a particular developer workload or toolchain inside an Environment. In particular, Core must not require or assume `containerd`, `nerdctl`, Docker CLI/Engine, an OCI registry, GitHub, cloud-provider CLIs, VS Code, or another IDE.

An Environment may contain those tools because a Base/Seed, operator, or optional plugin chose to provide them.

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

Core domain values must not import Incus, Git, GitHub, VS Code, Daintree, Rookery, OCI/container tooling, storage-backend, or cloud-provider implementation packages.

## Optional OCI plugin

Container tooling is an optional developer-workload integration implemented under `modules/plugin/oci`.

The OCI plugin may provide profiles backed by `nerdctl` or the genuine Docker CLI, OCI usage telemetry, Seed recommendations, local-registry helpers, and Docker Engine compatibility packaging. Those are plugin responsibilities even when a project-maintained development image enables them by default for convenience.

The plugin is opt-in at host composition time:

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
unset HACO_PLUGIN_OCI
```

With `HACO_PLUGIN_OCI` unset, Core must still support Environment lifecycle and execution without probing for `containerd`, `nerdctl`, Docker CLI, or Docker Engine.

Plugin-owned CLI functionality lives under:

```text
haco plugin oci ...
```

Core Base inspection remains under `haco base ...` because that command describes Hacocoon Environment Bases, not OCI workload images.

## Release placement

```text
v0.1-v0.6
  local runtime, workspace, policy/capability and orchestrator foundation

v0.7
  provider-neutral routing seam
  cloud implementation deferred
  previous EC2/AWS/EBS implementation retained only in Git history/design

v0.8-v0.12
  client adapters, per-agent binding, Agent Host, Bases and resource budgets

v0.13+
  optional OCI plugin features such as telemetry, Seed recommendation,
  deletion/tombstones and Docker compatibility
```

The first Incus implementation does not by itself require a generalized provider framework. The provider seam remains so a future remote/cloud adapter can be restored once local contracts stabilize.

## Do not over-generalize

Create interfaces only when a second implementation, test boundary, or replacement requirement makes them useful. Advanced storage or OCI code present in the repository is optional implementation inventory, not proof that storage or container tooling belongs in Core.

## Workspace ownership rule

A workspace is opaque to the runtime. If Daintree, Rookery, VS Code, a shell script, or a human created the directory/worktree, Hacocoon accepts the path without taking ownership of the upstream Git workflow.

An optional Git-worktree WorkspaceProvider, when implemented, is convenience functionality for standalone Hacocoon use—not the definition of a Workspace.

## External orchestrators

Orchestrators are clients above Hacocoon, not plugins inside Core. They may call the CLI, a future MCP adapter, or another stable protocol. Hacocoon never depends on their task DAG, model selection, retries, or budget model.
