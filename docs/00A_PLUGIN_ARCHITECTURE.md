# Adapter and Extension Architecture

Status: cross-cutting architecture reference. This is **not** a v0.1 implementation requirement.

## Goal

Hacocoon keeps a small Core while allowing concrete runtime, workspace, capability, approval, storage, and client integrations to evolve independently.

The design uses ordinary Go package boundaries and ports/adapters first. A dynamic plugin framework is added only if deployment or third-party extension needs justify it.

## Core ports

Expected long-term seams:

```text
WorkspaceProvider
EnvironmentProvider
CommandExecutor
CapabilityProvider
PolicyEvaluator
ApprovalProvider
EventSink
```

Core domain values should not import Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, Btrfs, QCOW2, or EC2 packages.

## Initial adapters

```text
Workspace
  ExternalPathWorkspace   <- v0.1
  DirectoryWorkspace      <- v0.2
  GitWorktreeWorkspace    <- optional v0.2+

Environment
  IncusEnvironment        <- v0.1
  EC2Environment          <- v0.7+

Approval
  CLIApproval             <- v0.4
  external/UI adapters    <- later

Capability
  GitHubCapability        <- v0.5
  AWSCapability           <- v0.7
```

## Do not over-generalize v0.1

v0.1 should not create interfaces merely because the roadmap names future providers. Start with the smallest concrete Incus/external-path vertical slice. Formalize a port when a second implementation or a stable test seam makes the abstraction useful.

## Workspace ownership rule

A workspace is opaque to the runtime. If Daintree, Rookery, VS Code, a shell script, or a human created the directory/worktree, Hacocoon accepts the path without taking ownership of the upstream Git workflow.

`GitWorktreeWorkspace`, when implemented, is a convenience provider for standalone Hacocoon use—not the definition of a Workspace.

## External orchestrators

Orchestrators are clients, not plugins inside Core. They may call the CLI, a future MCP adapter, or another stable protocol. Hacocoon never depends on their task DAG, model selection, retries, or budget model.
