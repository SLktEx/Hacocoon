# Adapter and Extension Architecture

Status: cross-cutting architecture reference. This is **not** a v0.1 implementation requirement.

## Goal

Hacocoon keeps a small Core while concrete environment, workspace, capability, approval, storage, and client integrations evolve independently.

Use ordinary Go package boundaries and ports/adapters first. A dynamic plugin framework is added only if deployment or third-party extension needs justify it.

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
```

Promote a seam into a Go interface when a second implementation, a stable test boundary, or a real replacement requirement makes it useful.

Core domain values must not import Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, storage-backend, or cloud-provider implementation packages.

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

v0.7+
  AWS capability adapter
  additional Environment implementation such as EC2
  storage adapters only where required by an Environment implementation
```

The first Incus implementation does not by itself require a generalized `EnvironmentProvider` framework. The second real environment backend is the natural point to validate that seam.

## Do not over-generalize v0.1

v0.1 should not create interfaces merely because the roadmap names future providers. Start with the smallest concrete Incus/external-path vertical slice.

Advanced storage code already present in the repository is historical implementation inventory, not proof that storage belongs in Core or in the v0.1 architecture.

## Workspace ownership rule

A workspace is opaque to the runtime. If Daintree, Rookery, VS Code, a shell script, or a human created the directory/worktree, Hacocoon accepts the path without taking ownership of the upstream Git workflow.

An optional Git-worktree WorkspaceProvider, when implemented, is convenience functionality for standalone Hacocoon use—not the definition of a Workspace.

## External orchestrators

Orchestrators are clients above Hacocoon, not plugins inside Core. They may call the CLI, a future MCP adapter, or another stable protocol. Hacocoon never depends on their task DAG, model selection, retries, or budget model.
