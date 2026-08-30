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
- **Standard** contains project-maintained, replaceable default implementations used by normal installations, such as the current Incus backend and future default egress enforcement.
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
| v0.17 | OCI Seed Builder & Btrfs/COW | first repository slice / partial |
| v0.18 | Docker Compatibility Plugin | repository implementation complete; real-host acceptance remains host-dependent |

Fully implemented product milestones are contiguous through **v0.16** because v0.17 remains partial. v0.18 has a complete repository implementation even though the preceding Seed/COW gate still has lifecycle/acceptance work.

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

v0.17 has a **first repository slice** implementing trusted Host acquisition/cache, an offline no-NIC Seed Builder, immutable Seed publication/current pointer, exact-parent resolution, and normal Incus/storage-driver cloning. Physical Btrfs COW measurement, conservative old-revision GC/recovery, authenticated/private-registry combinations, and broader real-host acceptance remain pending. Never share one writable `/var/lib/containerd` across Environments.

See [`../design/oci-seed-and-cow.md`](../design/oci-seed-and-cow.md), [`../design/docker-compatibility-plugin.md`](../design/docker-compatibility-plugin.md), and [`../OPTIONAL_LOCAL_OCI_REGISTRY.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.md).

## Client direction

Clients use generic Hacocoon contracts rather than becoming Core dependencies. `pkg/clientadapter` provides Environment/access operations and composes `pkg/interaction` for client-neutral event observation. VS Code is the first convenience client; code-server, JetBrains, browser UIs, and future clients can reuse the same boundaries.

## Numbering rule

One independently useful product feature is approximately one minor milestone. Security fixes, bug fixes, hardening, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only work normally do not consume another product version.

## Historical note

Old commits, branches, PRs, and document versions may use superseded milestone assignments or describe removed cloud implementations. Git history is the archive for those states; they do not override the current status/version authority.
