# Architecture and roadmap

> **Architecture baseline · Updated 2026-09-05**
>
> Hacocoon is a **Secure Workspace Runtime**. Use [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) for current code reality and [`versioning-and-release-status.md`](versioning-and-release-status.md) for authoritative development-checkpoint numbering/history.

This document describes product boundaries and forward direction. It intentionally does **not** duplicate the current checkpoint table or the full implementation-status matrix.

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

On the supported local path, the Physical Host remains the authority for Incus and privileged platform operations while a persistent trusted logical `haco-host` provides the normal management entry point. Incus owns the local Btrfs backing-image, loop, filesystem, and mount lifecycle. `haco-host` is TCB infrastructure, not an untrusted Environment.

The provider-neutral routing seam remains, but **cloud implementation is currently deferred**. Concrete EC2/AWS/EBS code is intentionally absent from the active tree.

Container tooling is also not Core. Optional OCI plugins may provide nerdctl/Docker behavior; with `HACO_PLUGIN_OCI` unset, Core remains usable without containerd, nerdctl, Docker, or a local Registry.

## Product boundaries

Hacocoon owns Workspace identity and leases, Environment lifecycle, execution, client-access primitives, ResourceBudget, Policy/Approval/Audit, narrow capabilities, interaction contracts, trusted Host management boundaries, and trusted session-to-Environment binding.

Hacocoon does not own IDE/AI chat UX, model routing, task DAGs, Git worktree orchestration, a mandatory OCI runtime, or a mandatory local Registry. Provider, client, and developer-tool specifics stay behind adapters, Standard implementations, or Plugins.

## Core, Standard, Plugin

- **Core** owns stable product semantics and security boundaries.
- **Standard** contains project-maintained, replaceable default implementations used by normal installations, including the current Incus backend and hostname-aware egress enforcement.
- **Plugin** contains optional or specialized integrations, including nerdctl/Docker/OCI tooling.

See [`../design/plugin-architecture.md`](../design/plugin-architecture.md) and [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md).

## Roadmap model

The immediate order is Windows installation → WSL Incus/storage/network → trusted `haco-host` → product `haco`. Existing Linux CI remains substrate validation. WSL is the current delivery target; macOS work is deferred and outside this scope. Finish standard SSH before adding IDE/open convenience. The Physical Host controller owns management state, Policy, providers and ownership; the trusted Host must not acquire another controller or Incus daemon.

Later Workspace work is **planned**: multiple independent COW repo clones, each with its own `.git`, and Workspace data retained across ordinary Environment deletion and Base changes, including uncommitted, untracked and unpushed work. Do not share the trusted Host's writable Git management area with Environments. Git remote operations are planned through `git-remote-haco`, with trusted execution and approval bound to repo, remote, ref, old/new OID and operation.

Development checkpoints are chronological progress markers, not roadmap phases that must all close before later work starts. The complete checkpoint history and current number live only in [`versioning-and-release-status.md`](versioning-and-release-status.md).

The roadmap is organized by architectural direction instead of copying per-checkpoint implementation status:

- strengthen the trusted Host/controller boundary while keeping untrusted Environments free of Host management authority;
- make the persistent logical `haco-host` the normal local/WSL operating surface without moving raw Incus authority into it;
- keep client integrations reusable and client-neutral, including interaction events, notification delivery, VS Code, browser, and future IDEs;
- preserve provider-neutral Environment/Core contracts while keeping concrete cloud backends deferred until local contracts settle;
- keep OCI/container tooling optional and separate from Core;
- keep local rootfs storage on one Incus-owned Btrfs/COW lifecycle and strengthen real-host acceptance around it;
- continue tightening real-host acceptance, especially Windows/WSL, networking, storage behavior, and client integration.

**Local OCI Registry is not a required roadmap gate.** It remains deferred optional infrastructure and may be reconsidered only if measured bandwidth, rate-limit, restricted-network, or centralized-policy needs justify it.

## Trusted Host direction

On the local Incus/WSL path, Hacocoon distinguishes the **Physical Host** from the persistent trusted logical **`haco-host`**. The Physical Host retains Incus and platform authority; Incus itself owns the Btrfs pool backing, loop, filesystem, and mount lifecycle. `haco-host` is trusted infrastructure inside the TCB, not an untrusted Environment.

The implemented lifecycle/default-entry slice does not expose the raw Incus control socket. Follow-up work should move ordinary Hacocoon operations toward the logical Host through narrow controller/client contracts while preserving explicit Physical-Host recovery and bootstrap paths.

See [`../design/trusted-host.md`](../design/trusted-host.md) and [`../WINDOWS_WSL_BOOTSTRAP.md`](../WINDOWS_WSL_BOOTSTRAP.md).

## Base and OCI separation

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci ...
HACO_PLUGIN_OCI=docker   haco plugin oci ...
```

`haco base` describes Environment starting identity. OCI/container lifecycle is an optional Plugin responsibility. The maintained OCI profile may use containerd + nerdctl, and Docker compatibility may use genuine Docker CLI plus Environment-local socket-activated Engine; neither is a Core invariant.

## OCI storage direction

Seed retirement is **planned**; code and pinned revisions remain. Separate Base resolution from the optional Seed resolver before removing the retained commands/builders and acceptance surfaces. Base choice and optional OCI Plugin structure remain. The [Seed design](../design/oci-seed-and-cow.md) owns the dependency/removal boundary. Never share one writable `/var/lib/containerd` across Environments.

Local rootfs storage routes Hacocoon-owned Base, Tooling, Seed, trusted-host, and Environment rootfs paths through `haco-local-default`, an Incus-owned loop-backed Btrfs pool, rather than inheriting an unrelated Host default pool. Pool creation requests `compress=zstd:3,noatime,nodiscard`; `compress-force` and `autodefrag` are intentionally not desired defaults, and Hacocoon does not automatically rewrite old extents because doing so could reduce reflink/COW sharing.

The ordinary CLI remains non-root. Hacocoon asks Incus to provide the storage pool through the normal runtime boundary and does not implement a separate block-device or mount lifecycle.

See [`../design/oci-seed-and-cow.md`](../design/oci-seed-and-cow.md), [`../design/btrfs-storage-layout.md`](../design/btrfs-storage-layout.md), [`../design/docker-compatibility-plugin.md`](../design/docker-compatibility-plugin.md), and [`../OPTIONAL_LOCAL_OCI_REGISTRY.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.md).

## Egress direction

Hostname-aware egress keeps the authorization contract in Core and concrete default HTTP/HTTPS proxy enforcement in Standard. The Host resolves and pins public destinations only after authorization; the Incus path remains proxy-only at the lower transport layer. Real supported-Incus bridge/nftables/dnsmasq behavior remains an acceptance concern rather than a reason to duplicate status here.

See [`../EGRESS_AUTHORIZATION.md`](../EGRESS_AUTHORIZATION.md).

## Client direction

Clients use generic Hacocoon contracts rather than becoming Core dependencies. `pkg/clientadapter` provides Environment/access operations and composes `pkg/interaction` for client-neutral event observation. Browser/native notifications and the optional VS Code notification extension consume the same minimized event boundary; observation/delivery never becomes an authorization path.

Standard SSH is the first complete connection path. VS Code, code-server, JetBrains, browser UIs, and future clients should reuse the same boundaries rather than introducing client-specific authority into Core.

See [`../CLIENT_ADAPTER_CONTRACT.md`](../CLIENT_ADAPTER_CONTRACT.md) and [`../INTERACTION_EVENTS.md`](../INTERACTION_EVENTS.md).

## Operational confidence direction

Real-Incus CI acceptance proves the substrate independently before Core lifecycle checks, making substrate failures distinguishable from Hacocoon regressions. Incus-owned storage acceptance exercises the ordinary-user CLI against real Incus and verifies backing-image, loop, Btrfs mount, compression policy, pool reuse, and guarded cleanup; trusted-host acceptance verifies lifecycle/ownership/control-socket isolation.

Structured logging uses `log/slog`, stable operation context, sanitized Host-command diagnostics, and defense-in-depth secret redaction across maintained executables. See [`../reference/logging.md`](../reference/logging.md).

These operational checkpoints improve support confidence and diagnosability without claiming universal Host support. Exact current acceptance remains in [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md).

## Numbering rule

Minor versions are pragmatic pre-1.0 progress checkpoints. Meaningful product, implementation, operator-experience, observability, or acceptance slices may take the next minor even when follow-up work or real-host acceptance remains. Small fixes and maintenance do not automatically consume another version, but substantial support/operability checkpoints may. During pre-1.0 development, visible progression is preferred over conserving minor numbers.

Published tags/releases and acceptance/support evidence are separate concepts. See [`versioning-and-release-status.md`](versioning-and-release-status.md).

## Historical note

Old commits, branches, PRs, and document versions may use superseded checkpoint assignments or describe removed cloud implementations. Git history is the archive for those states; it does not override the current status/version authority.
