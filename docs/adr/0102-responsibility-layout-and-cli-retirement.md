# Responsibility layout and CLI retirement

Status: accepted. Refs #654. Based on main `ee8bf7fbd2e858f9b6fd1d22ecce77fdde6bf082`.

## Decision

Repository paths identify the feature or external system that owns the code.
Core, Standard and Plugin describe architectural roles, not parallel directory
trees. Use ordinary Go packages and explicit composition; do not add compatibility
aliases for moved internal packages. [The repository map](../../CONTRIBUTING.md#repository-map)
owns navigation; [extension architecture](../design/plugin-architecture.md) owns
the architectural rules.

All shipped binaries retain independent thin entries in `cmd/`. The current
product is built from `cmd/haco`, with parsing and presentation in `internal/cli`.
The previous `hacoq` binary is removed from source, release archives and installers.
Its direct GitHub capability, optional Docker status/prepare service and
`HACO_PLUGIN_OCI` composition selector are retired. Current managed Git,
OCI Store/image operations, controller APIs, notifications and client adapters
remain separate supported implementations. Generic capability/event APIs remain
because current client and notification consumers use them.

Installer source moves from `scripts/` to `install/`. Direct source URLs change;
there are no forwarding scripts. Release archive names and installer bundle
filenames stay the same. The installer rejects archives containing the retired
binary. This change does not erase an old installation's files or data.

## Boundaries retained

Base build, asset retention and reviewed image cleanup remain separate packages.
Env routing, copy and transfer remain distinct from snapshot restore and the
canonical Workspace lifecycle. A saved snapshot is not owned by a live Env.
SSH configuration and public-key validation have distinct clients and provider
consumers; moving them does not give a workload management authority.

Git process/wire code belongs to its adapter. Repository ownership and approval
coordination remain with the Git feature. OCI download/extraction belongs to its
adapter; retained Store ownership remains with storage services. Windows/WSL
identity, enrollment and worker code stays separate from reclamation values and
the client that dispatches it. All creation receipts, exact-owner cleanup,
approval and isolation checks still apply across the new paths.

Existing retained Base/snapshot ownership records still need verified cleanup.
Those readers and cleanup checks are not disposable aliases: removing them would
orphan owned resources or invite adoption based on a name. They remain until their
data-lifetime contract is explicitly replaced.

## Rejected alternatives

- Keeping both CLI implementations or wrapping the old one from the product:
  this preserves two compositions and an obsolete authority surface.
- Merging helpers to reduce binary count: helpers have different execution
  locations, platforms and authority.
- Moving every Standard implementation into adapters: approval, cache selection
  and Workspace orchestration are product behavior, not external-system calls.
- Combining Env, snapshot and Workspace mutation services: their data lifetimes
  differ and canonical lifecycle transitions must remain the sole ownership writer.
- Claiming native acceptance from repository tests: packaged Ubuntu/Windows,
  Incus and authenticated service acceptance remain separate checks.
