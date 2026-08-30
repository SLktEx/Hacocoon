# Architecture and roadmap

> **Architecture baseline · Updated 2026-08-30**
>
> Hacocoon is a **Secure Workspace Runtime**. Use [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) for current code reality and [`versioning-and-release-status.md`](versioning-and-release-status.md) for authoritative milestone numbering.

Hacocoon gives developer tools and coding agents broad freedom inside isolated Environments while keeping Host and external authority behind explicit trusted boundaries.

## Current shape

```text
Client / IDE / Agent / Orchestrator
              |
           Workspace
              v
       +-------------+
       |  Hacocoon   |
       | Environment |
       | Policy      |
       | Capability  |
       +------+------+ 
              |
      provider / adapter
              |
          Incus (current)
```

The provider-neutral v0.7 routing seam remains, but **cloud implementation is currently deferred**. Concrete EC2/AWS/EBS code is intentionally absent from the active tree.

Container tooling is also not Core. Optional OCI plugins may provide nerdctl/Docker behavior; with `HACO_PLUGIN_OCI` unset, Core remains usable without containerd, nerdctl, Docker, or a local Registry.

## Product boundaries

Hacocoon owns Workspace identity and leases, Environment lifecycle, execution, client-access primitives, ResourceBudget, Policy/Approval/Audit, narrow capabilities, interaction contracts, and trusted session-to-Environment binding.

Hacocoon does not own IDE/AI chat UX, model routing, task DAGs, Git worktree orchestration, a mandatory OCI runtime, or a mandatory local Registry. Provider, client, and developer-tool specifics stay behind adapters, Standard implementations, or Plugins.

## Core, Standard, Plugin

- **Core** owns stable product semantics and security boundaries.
- **Standard** contains project-maintained, replaceable default implementations used by normal installations, including the current Incus backend and hostname-aware egress enforcement.
- **Plugin** contains optional or specialized integrations, including nerdctl/Docker/OCI tooling.

See [`../design/plugin-architecture.md`](../design/plugin-architecture.md) and [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md).

## Roadmap

| Version | Gate | Repository status |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | implemented |
| v0.2 | Workspace Abstraction & Lease | implemented |
| v0.3 | Client & Interactive Access | implemented |
| v0.4 | Policy & Capability Foundation | implemented |
| v0.5 | Git / GitHub Capability | implemented |
| v0.6 | Agent & Orchestrator Integration | implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | provider routing seam retained; concrete cloud deferred |
| v0.8 | Client Adapters & VS Code Integration | implemented |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | implemented |
| v0.11 | Base Images & Custom Environments | first slice implemented |
| v0.12 | Sandbox Resource Limits | first slice implemented |
| v0.13 | Managed Sandbox Network | implemented |
| v0.14 | Git Fetch Plugin | implemented |
| v0.15 | OCI Seed Recommendation | implemented |
| v0.16 | OCI Image Deletion | first slice implemented |
| v0.17 | OCI Seed Builder & Btrfs/COW | build/publish + operations-hardening repository slices / partial |
| v0.18 | Docker Compatibility Plugin | repository implementation complete; real-host acceptance remains host-dependent |
| v0.19 | Domain-aware Egress Authorization | repository implementation complete; real supported-Incus acceptance remains host-dependent |
| v0.20 | Managed Btrfs Rootfs Storage | first repository slice implemented; physical COW/compaction acceptance remains host-dependent |
| v0.21 | Managed Btrfs Transparent Compression | `compress=zstd:3` managed default implemented; real compression/performance acceptance remains host-dependent |

The current milestone position is **v0.21**. Milestones are lightweight pre-1.0 development checkpoints, so a partial earlier gate does not block later progress.

**Local OCI Registry is not a roadmap milestone.** It remains deferred optional infrastructure and may be reconsidered only if measured bandwidth, rate-limit, restricted-network, or centralized-policy needs justify it.

## Base and OCI separation

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci ...
HACO_PLUGIN_OCI=docker   haco plugin oci ...
```

`haco base` describes Environment starting identity. OCI/container lifecycle is an optional Plugin responsibility. The maintained OCI profile may use containerd + nerdctl, and Docker compatibility may use genuine Docker CLI plus Environment-local socket-activated Engine; neither is a Core invariant.

## OCI storage direction

v0.17 repository work covers trusted Host acquisition/cache, an offline no-NIC Seed Builder, immutable Seed publication/current pointer, exact-parent resolution, explicit per-Base immutable pins, exact re-enable after deletion, conservative old-revision GC, interrupted-builder recovery, deletion-race protection, credential-free managed-Environment harvest, and normal Incus/storage-driver cloning. Authenticated/private-registry combinations, physical Btrfs COW measurement, broader real-host failure injection, and supported-host acceptance remain pending. Never share one writable `/var/lib/containerd` across Environments.

v0.20 extends the storage boundary to all Hacocoon-owned local Incus rootfs paths. Local composition lazily ensures one sparse-raw Btrfs filesystem per configured Hacocoon storage pool and routes Base, Tooling, Seed, Environment rootfs volumes, snapshots, and clones through its `haco-<storage-id>` Incus pool rather than inheriting the Host default pool.

v0.21 standardizes managed transparent compression. Managed Btrfs mounts use `compress=zstd:3`; non-compliant managed mounts are remounted, `compress-force` is intentionally not the desired state, and Hacocoon does not automatically rewrite old extents because that could reduce reflink/COW sharing. Physical compression ratio, CPU cost, COW behavior, and compaction remain host-dependent acceptance concerns.

See [`../design/oci-seed-and-cow.md`](../design/oci-seed-and-cow.md), [`../design/btrfs-storage-layout.md`](../design/btrfs-storage-layout.md), [`../design/docker-compatibility-plugin.md`](../design/docker-compatibility-plugin.md), and [`../OPTIONAL_LOCAL_OCI_REGISTRY.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.md).

## Egress direction

v0.19 records the repository-complete hostname-aware egress slice: Core authorization, the Standard HTTP/HTTPS proxy, Host-side DNS pinning and address filtering, CONNECT/SNI validation, Incus proxy-only transport enforcement, trusted source-IP Environment mapping, and `haco egress serve`. Real supported-Incus bridge/nftables/dnsmasq acceptance remains host-dependent. See [`../EGRESS_AUTHORIZATION.md`](../EGRESS_AUTHORIZATION.md).

## Client direction

Clients use generic Hacocoon contracts rather than becoming Core dependencies. `pkg/clientadapter` provides Environment/access operations and composes `pkg/interaction` for client-neutral event observation. VS Code is the first convenience client; code-server, JetBrains, browser UIs, and future clients can reuse the same boundaries.

## Numbering rule

Minor versions are pragmatic pre-1.0 progress checkpoints. Meaningful implementation slices may take the next minor even when follow-up work or real-host acceptance remains. Security fixes, bug fixes, hardening, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only work normally do not consume another product version by themselves.

## Historical note

Old commits, branches, PRs, and document versions may use superseded milestone assignments or describe removed cloud implementations. Git history is the archive for those states; they do not override the current status/version authority.
