# Adapter and Extension Architecture

Status: cross-cutting architecture reference.

## Goal

Hacocoon keeps a small Core while concrete environment, capability, client and optional developer-tool integrations evolve independently.

Use ordinary Go package boundaries and static composition first. `plugin` describes a product/ownership boundary; it does **not** imply a Go shared-object loader. Dynamic loading may be added later only if deployment or third-party extension needs justify it.

## Core boundary

Core domain vocabulary stays small:

```text
Workspace
WorkspaceLease
Environment
Execution
CapabilityRequest
PolicyDecision
ApprovalRequest
BaseName / BaseRevision / BaseRef
ResourceBudget
```

Core must not import or require concrete Incus, EC2, GitHub, VS Code, nerdctl, Docker, OCI Registry or Btrfs mechanics as domain concepts.

## Adapter vs optional plugin

Two extension shapes are intentionally distinguished.

### Provider / adapter

A provider implements a Hacocoon-owned abstraction such as an Environment backend or client adapter.

Examples:

- Incus Environment provider
- experimental EC2 Environment provider
- VS Code client adapter

### Optional feature plugin

A plugin adds integration-specific user functionality that is not required for Hacocoon Core to operate.

Current examples:

```text
haco plugin git ...
haco plugin oci ...
```

Git/GitHub-specific repository/ref/auth behavior and OCI-container-tool behavior belong to these extension surfaces, while generic Policy/Capability and Environment execution contracts remain in Core.

## Git plugin

```text
haco plugin git fetch <environment>
haco plugin git push <environment> --branch <branch>
```

Ordinary Git UX remains Git's responsibility. Hacocoon owns only the privileged Host-side capability boundary for operations such as brokered fetch/push.

## OCI plugin

The OCI plugin is explicit opt-in:

```text
HACO_PLUGIN_OCI=nerdctl
# or
HACO_PLUGIN_OCI=docker
```

When the variable is unset, the OCI service is not composed. Hacocoon Core must not require nerdctl, Docker CLI, dockerd, a Host OCI image cache, or a Local Registry simply to manage Environments.

Plugin commands include:

```text
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

A project-maintained `containerd + nerdctl` profile is useful for installations that want it, but it is not a universal Hacocoon runtime requirement. Docker compatibility is another optional profile/interface.

`haco image list|inspect` remains separate: those commands describe Hacocoon Base identity rather than workload OCI image management.

## Candidate seams

Long-term conceptual seams include:

```text
WorkspaceProvider
EnvironmentProvider
CommandExecutor
CapabilityProvider
PolicyEvaluator
ApprovalProvider
EventSink
```

Promote a seam into a Go interface when a second implementation, a stable test boundary, or a real replacement requirement makes it useful. Do not create interfaces merely because a roadmap names a possible future provider.

## Environment providers

The first Incus implementation did not require a generalized provider framework by itself; EC2 later validated the provider seam. EC2 remains experimental/default-off and must not trigger AWS credential lookup or network activity while disabled.

## Workspace ownership

A Workspace is opaque to the runtime. If an orchestrator, IDE, shell script or human created a directory/worktree, Hacocoon accepts the path without taking ownership of the upstream Git workflow.

## External orchestrators

Orchestrators are clients above Hacocoon, not plugins inside Core. They may own task decomposition, branch/worktree creation, parallelism, retry, model selection and development review. Hacocoon owns the isolated Environment and security boundary underneath them.

## Rule of thumb

> If Hacocoon can still provide secure Workspace/Environment execution without a tool, that tool-specific workflow should not become a Core dependency merely because one deployment wants it.
