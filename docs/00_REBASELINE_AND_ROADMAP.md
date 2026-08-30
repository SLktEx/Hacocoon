# Architecture & Roadmap

> **Architecture baseline · Updated 2026-08-30**
>
> Hacocoon is a **Secure Workspace Runtime**. Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for current code reality and [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) for authoritative milestone numbering.

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
        provider/adapter
              |
          Incus (current)
```

The provider-neutral v0.7 routing seam remains, but **cloud implementation is currently deferred**. Concrete EC2/AWS/EBS code is intentionally absent from the active tree.

Container tooling is also not Core. An optional OCI plugin may provide nerdctl/Docker behavior; with `HACO_PLUGIN_OCI` unset, Core must remain usable without containerd, nerdctl, Docker, or a local Registry.

## Product boundaries

Hacocoon owns Workspace identity and leases, Environment lifecycle, execution, client-access primitives, ResourceBudget, Policy/Approval/Audit, narrow capabilities, and trusted per-session Environment binding.

Hacocoon does not own IDE/AI chat UX, model routing, task DAGs, Git worktree orchestration, a mandatory OCI runtime, or a mandatory local Registry. Provider/client/container specifics stay behind adapters/plugins.

## Roadmap

| Version | Gate | Repository status |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | implemented |
| v0.2 | Workspace Abstraction & Lease | implemented |
| v0.3 | Client & Interactive Access | implemented |
| v0.4 | Policy & Capability Foundation | implemented |
| v0.5 | Git / GitHub Capability | implemented |
| v0.6 | Agent & Orchestrator Integration | implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | provider-neutral routing seam retained; concrete cloud implementation deferred |
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

Fully implemented product milestones are contiguous through **v0.16** because v0.17 remains partial. The v0.18 Docker repository implementation landed early under the previous numbering and is retained.

**Local OCI Registry is not a roadmap milestone.** It remains deferred optional infrastructure and may be reconsidered only if measured bandwidth, rate-limit, restricted-network, or centralized-policy needs justify it.

## Numbering rule

One independently useful product feature is approximately one minor milestone. Security fixes, bug fixes, hardening, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only work normally do not consume another product version.

## Base and OCI separation

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci ...
HACO_PLUGIN_OCI=docker   haco plugin oci ...
```

`haco base` describes Environment starting identity. OCI/container lifecycle is an optional plugin responsibility. The project-maintained plugin profile may use containerd + nerdctl, and Docker compatibility may use genuine Docker CLI plus Environment-local socket-activated Engine; neither is a Core invariant.

The Docker lifecycle commands are assigned to v0.18: `haco plugin oci docker status <environment>` and `prepare <environment>`. `prepare` validates the Base-provided Docker compatibility profile and pinned systemd units, does not install packages, and refuses to silently stop an already-active vendor Docker daemon. Their code landed before this roadmap reorder while Docker Compatibility was numbered v0.17.

## OCI storage direction

Normal upstream pull is allowed by policy. Local Registry is optional/deferred and is not required for Seed construction. v0.17's **first repository slice** implemented trusted Host acquisition/cache, an offline no-NIC Seed Builder, immutable Seed publication/current pointer, exact-parent resolution, and normal Incus/storage-driver cloning. The operations-hardening slice adds per-Base immutable pins, exact tombstone re-enable, conservative old-revision GC, interrupted-builder recovery before builds, and deletion-race protection. Physical Btrfs COW measurement, authenticated/private-registry combinations, and broader real-host acceptance remain pending. Never share one writable `/var/lib/containerd` across Environments.

See [`17_v0.17_OCI_SEED_AND_COW.md`](17_v0.17_OCI_SEED_AND_COW.md), [`18_v0.18_DOCKER_COMPATIBILITY_PLUGIN.md`](18_v0.18_DOCKER_COMPATIBILITY_PLUGIN.md), and [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md).

## Client interaction direction

Browser/Web interaction and notification work belongs at a client/adapter boundary. A VS Code extension may provide optional IDE-native UX, but Core should expose generic interaction contracts rather than owning an IDE UI.

## Historical note

Older commits, branches, PRs, and superseded documents may describe active EC2/AWS/EBS support or place Local Registry / Seed / deletion work under older milestone assignments. Those are historical records, not the current architecture or numbering.

A short-lived 2026-08-30 rebaseline reserved v0.18 for Optional Local OCI Registry and v0.19 for OCI Seed Builder/COW. That reservation is superseded: Registry is deferred/unversioned.

A subsequent ordering placed Docker Compatibility at v0.17 and Seed Builder/COW at v0.18, and both repository implementations began landing under that numbering. The authoritative order is now v0.17 Seed Builder/COW followed by v0.18 Docker Compatibility; the already-landed code remains intact and is reclassified rather than rolled back.
