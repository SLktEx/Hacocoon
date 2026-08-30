# Adapter and Extension Architecture

Status: cross-cutting architecture reference. This is **not** a v0.1 implementation requirement.

## Goal

Hacocoon keeps a small Core while concrete environment, workspace, capability, approval, storage, client, and developer-tool integrations evolve independently.

Use ordinary Go package boundaries and ports/adapters first. A dynamic plugin framework is added only when deployment or third-party extension needs justify it. The CLI `haco plugin ...` namespace is a user-facing extension boundary; it does not imply that every plugin must be a dynamically loaded shared object.

## Core boundary

Core owns Environment lifecycle, isolation, execution, connection management, generic policy/approval boundaries, and events.

Core must not require a particular developer workload or toolchain inside an Environment. In particular, Core must not require or assume:

- `containerd`;
- `nerdctl`;
- Docker CLI or Docker Engine;
- an OCI registry;
- OCI Seed telemetry;
- Git/GitHub tooling;
- a cloud-provider CLI;
- VS Code or another IDE.

A Base/Seed, operator, or optional plugin may provide those tools without making them Core dependencies.

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

Core domain values must not import Incus, Git, GitHub, VS Code, OCI/container tooling, storage-backend, or cloud-provider implementation packages.

## Optional OCI plugin

OCI/container-specific behavior is implemented under `modules/plugin/oci` and is composed explicitly:

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
unset HACO_PLUGIN_OCI
```

`nerdctl` and `docker` select the CLI driver used by the optional plugin for OCI inventory/telemetry. Selecting a driver does not install that binary, grant credentials, or authorize an arbitrary Host Docker daemon.

With `HACO_PLUGIN_OCI` unset, Hacocoon Core must remain usable without probing for or requiring `containerd`, `nerdctl`, Docker CLI, or Docker Engine.

Plugin-owned CLI functionality lives under:

```text
haco plugin oci ...
```

Core Base inspection remains under `haco base ...` because it describes Hacocoon Environment starting points, not OCI workload images.

## Current milestone placement

```text
v0.7
  provider-neutral routing seam
  cloud implementation deferred

v0.13
  managed sandbox network

v0.14
  Git fetch plugin

v0.15
  OCI Seed recommendation

v0.16
  OCI image deletion / tombstones

v0.17
  Docker compatibility plugin
  plugin boundary/driver composition present
  Docker Engine/Base integration and real-host acceptance still partial

v0.18-v0.19
  optional Local Registry and Seed Builder/COW follow-ups
```

The move of existing v0.15/v0.16 OCI implementation behind the plugin package is a boundary/refactor correction and does not consume another product milestone.

## Provider boundary

Incus is the only Environment provider registered by the current build. The provider-neutral seam remains so a remote/cloud adapter can be restored later. The previous concrete EC2/AWS/EBS implementation is deliberately absent from the active tree; **cloud implementation is currently deferred** while local/provider contracts stabilize.

## Do not over-generalize

Create interfaces only when a second implementation, a useful test boundary, or a replacement requirement makes them valuable. Optional OCI or historical storage code is not proof that container or storage tooling belongs in Core.

## Workspace ownership rule

A Workspace is opaque to the runtime. If an external tool, VS Code, a shell script, or a human created the directory/worktree, Hacocoon accepts the path without taking ownership of the upstream Git workflow.

## External orchestrators

Orchestrators are clients above Hacocoon, not plugins inside Core. They may call the CLI or a future stable protocol. Hacocoon never depends on their task DAG, model selection, retries, or budget model.
