# Adapter and extension architecture

Status: current architecture contract. [Design principles](../DESIGN_PRINCIPLES.md) explain the product goals; this document assigns implementation responsibilities.

## Core / Standard / Plugin classification

| Owner | Responsibility | Current examples |
|---|---|---|
| Core | Workspace/Environment lifecycle, Execution, Policy/Approval/Capability, consistent ownership and audit contracts | Generic requests and lifecycle transitions |
| Standard | Maintained default implementations of Core contracts; replaceable even when enabled by default | Incus backend, domain-aware HTTP/HTTPS egress proxy, notification adapters |
| Plugin | Optional workload integrations or alternative implementations | Git/GitHub, AWS operations, OCI tooling, specialized client integrations |

Frequency of use does not make an implementation Core. Ask whether removing a contract would change Hacocoon's product/security semantics, whether normal users need one replaceable implementation, or whether the feature is optional.

Use ordinary Go package boundaries and explicit composition. No dynamic shared-object loader is required. Add interfaces for a real replacement need, a stable test boundary or a security-critical contract. Names in future plans are not a checklist of interfaces to create.

## Provider boundary

Start from Incus capabilities and keep its calls behind the adapter. Do not constrain Incus, introduce recovery state or add Core conditionals for hypothetical backends. Current local storage is Incus-owned Btrfs; future backends must meet the same lifecycle and security invariants through their own mechanisms.

Core domain values must not import Incus, Git/GitHub, AWS, OCI, VS Code, storage-engine or orchestrator packages. Provider state remains opaque outside its owner. A provider must explicitly refuse a guarantee it cannot enforce; platform differences cannot silently weaken source identity, isolation or cleanup ownership.

A stronger VM/microVM backend is a possible future direction, not a currently selectable Hacocoon implementation. The [cloud runtime](remote-and-cloud-runtime.md) remains deferred.

## Egress ownership

```text
Environment request
  -> Core Policy / Approval / Capability
  -> Standard egress proxy
  -> provider-enforced network boundary
  -> authorized destination
```

Core defines the authority being requested and the decision. HTTP CONNECT, SNI validation, DNS resolution, nftables and Incus network configuration belong to their implementing Standard/provider components. The maintained proxy and hostname authorization are implemented; see [egress authorization](egress-authorization.md) for the exact contract and limits.

Specialized actions such as Git push need repository/ref/commit authority rather than a broad network grant. They reuse the same Policy/Approval semantics without making Git a Core dependency.

## Optional workload tooling

Docker Engine, containerd, nerdctl, OCI registries, Git, cloud CLIs and IDEs are not Core prerequisites. A Base, operator or plugin may provide them. Their absence must not disable ordinary Environment lifecycle, execution or Policy.

The retained legacy OCI composition is explicitly selected on the Physical Host:

```sh
export HACO_PLUGIN_OCI=nerdctl  # alternative: HACO_PLUGIN_OCI=docker
# Omit/unset HACO_PLUGIN_OCI for no legacy OCI plugin.
```

Legacy plugin commands use `hacoq plugin oci ...`; the current product exposes explicit operations such as `haco plugin oci store ...`. These are separate composition surfaces; consult [CLI migration](../reference/cli-migration.md). Base inspection uses `haco base ...` and describes Environment starting points, not OCI workload images.

[Persistent OCI Stores](persistent-oci-store.md) define current retained data. The [Seed implementation](oci-seed-and-cow.md) remains a legacy optional path pending retirement. Neither Docker compatibility nor Seed construction becomes mandatory because its maintained plugin exists.

## Workspace and client ownership

A runtime receives a resolved Workspace and enforced lease from its owner; it does not reconstruct Git or source-location policy. Ordinary product use creates independent managed repository volumes. The retained external-path adapter accepts an explicitly selected caller-owned path without adopting the caller's upstream Git workflow. See [Workspace contracts](workspace-abstraction-and-lease.md).

Clients own private SSH keys, launch behavior, editor configuration and presentation. Providers own connection creation and cleanup. [Client adapters](client-and-interactive-access.md) keep this separation without IDE conditionals in Core.

## External orchestrators

Orchestrators are clients above Hacocoon. Task graphs, model selection, retries, budgets and agent routing stay outside Core. They may use the CLI or a separately implemented adapter; a future MCP interface is not implied to exist.

Historical feature numbering is tracked in [release status](../status/versioning-and-release-status.md). Current implementation and acceptance belong in [implementation status](../IMPLEMENTATION_STATUS.md), not in a second roadmap here.
