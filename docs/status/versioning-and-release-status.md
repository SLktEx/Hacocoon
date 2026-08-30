# Versioning and release status

> **Authoritative milestone numbering · Updated 2026-08-31**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

Use this document for **which version number belongs to which development checkpoint**. Use [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

> **Minor milestones are intentionally cheap pre-1.0 progress checkpoints, not completeness gates.**

1. A durable product, implementation, operator-facing, or cross-cutting infrastructure step may consume the next `v0.N` milestone even when follow-up slices, hardening, or real-host acceptance remain.
2. A partial earlier milestone does not block later milestones. Version order is chronology, not a claim that every previous gate is complete.
3. Prefer advancing the number over packing unrelated durable capabilities into one milestone. Closely related work may still share one milestone.
4. Pure bug fixes, narrow hardening, refactors, CLI cleanup, docs, release engineering, and test-only work normally do not consume a milestone by themselves. A durable new operational contract such as notifications, logging, or a privileged broker may consume one.
5. Milestone changes update this file and `../IMPLEMENTATION_STATUS.md`; relevant roadmap/index summaries should remain aligned.
6. Design-only specifications may reserve future numbers but remain **planned** until implementation lands.
7. Historical commit messages, PR titles, candidate branches, old document addresses, and superseded numbering are not authoritative.
8. Tags/releases are separate from roadmap milestone numbering.

## Current authoritative numbering

**Status legend:** ✅ implemented · 🧪 partial / foundation · 🚧 planned/deferred

| Version | Gate | `main` status |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented |
| v0.3 | Client & Interactive Access | ✅ implemented |
| v0.4 | Policy & Capability Foundation | ✅ implemented |
| v0.5 | Git / GitHub Capability | ✅ implemented |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 provider routing seam retained; concrete cloud implementation deferred |
| v0.8 | Client Adapters & VS Code Integration | ✅ implemented |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented |
| v0.11 | Base Images & Custom Environments | ✅ first slice implemented |
| v0.12 | Sandbox Resource Limits | ✅ first slice implemented |
| v0.13 | Managed Sandbox Network | ✅ implemented |
| v0.14 | Git Fetch Plugin | ✅ implemented |
| v0.15 | OCI Seed Recommendation | ✅ implemented |
| v0.16 | OCI Image Deletion | ✅ first slice implemented |
| v0.17 | OCI Seed Builder & Btrfs/COW | 🧪 repository build/publish and operations-hardening slices implemented; real-host/private-registry/COW acceptance remains |
| v0.18 | Docker Compatibility Plugin | ✅ repository implementation complete; real-host acceptance tracked separately |
| v0.19 | Domain-aware Egress Authorization | ✅ repository implementation complete; real supported-Incus acceptance remains host-dependent |
| v0.20 | Managed Btrfs Rootfs Storage | ✅ first repository slice implemented; real-host COW/compaction acceptance remains host-dependent |
| v0.21 | Managed Btrfs Transparent Compression | ✅ `compress=zstd:3` default implemented without `compress-force`; real compression/performance acceptance remains host-dependent |
| v0.22 | Notification Delivery & Client Adapters | ✅ browser, native OS, and VS Code notification clients implemented and tested |
| v0.23 | Structured Logging & Diagnostic Safety | ✅ shared `log/slog`, structured diagnostics, and secret redaction implemented |
| v0.24 | Managed Host Privilege Broker for Btrfs Storage | ✅ root-owned narrow storage helper, privilege separation, and real normal-user Incus/Btrfs CLI acceptance implemented |

The current milestone position is **v0.24**. Earlier partial milestones remain visible as acceptance/work items but do not prevent later development checkpoints from advancing.

v0.7 keeps its number because its provider-neutral routing seam remains useful. Concrete EC2/AWS/EBS code is absent from the active tree and **cloud implementation is currently deferred**.

**Local Registry infrastructure is deferred and unversioned.** It is not required by normal pulls or Seed construction and does not reserve a roadmap milestone.

## Specification map

Document addresses are semantic and do not change when milestone assignments change.

| Gate | Design / reference document |
|---|---|
| v0.13 Managed Sandbox Network | [`../design/managed-sandbox-network.md`](../design/managed-sandbox-network.md) |
| v0.14 Git Fetch Plugin | [`../design/git-fetch-plugin.md`](../design/git-fetch-plugin.md) |
| v0.15 OCI Seed Recommendation | [`../design/oci-seed-recommendation.md`](../design/oci-seed-recommendation.md) |
| v0.16 OCI Image Deletion | [`../design/oci-image-deletion.md`](../design/oci-image-deletion.md) |
| v0.17 OCI Seed Builder & Btrfs/COW | [`../design/oci-seed-and-cow.md`](../design/oci-seed-and-cow.md) |
| v0.18 Docker Compatibility Plugin | [`../design/docker-compatibility-plugin.md`](../design/docker-compatibility-plugin.md) |
| v0.19 Domain-aware Egress Authorization | [`../EGRESS_AUTHORIZATION.md`](../EGRESS_AUTHORIZATION.md) |
| v0.20 Managed Btrfs Rootfs Storage | [`../design/btrfs-storage-layout.md`](../design/btrfs-storage-layout.md) |
| v0.21 Managed Btrfs Transparent Compression | [`../design/btrfs-storage-layout.md`](../design/btrfs-storage-layout.md) |
| v0.22 Notification Delivery & Client Adapters | [`../INTERACTION_EVENTS.md`](../INTERACTION_EVENTS.md) |
| v0.23 Structured Logging & Diagnostic Safety | [`../reference/logging.md`](../reference/logging.md) |
| v0.24 Managed Host Privilege Broker for Btrfs Storage | [`../design/btrfs-storage-layout.md`](../design/btrfs-storage-layout.md) |
| Optional Local OCI Registry | [`../OPTIONAL_LOCAL_OCI_REGISTRY.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.md) |

## Acceptance watch list

- **v0.7:** cloud implementation is deferred; acceptance will be redefined when a concrete cloud adapter returns.
- **v0.8-v0.13:** real Windows/WSL, Agent Host, Base/resource/network combinations remain partly host-dependent.
- **v0.14-v0.18:** repository behavior is implemented, with private repository/registry, physical COW, and Docker real-host combinations still acceptance-sensitive.
- **v0.19:** hostname-aware proxy authorization/enforcement is implemented; real supported-Incus bridge/nftables/dnsmasq acceptance remains host-dependent.
- **v0.20-v0.21:** managed Btrfs storage/compression are implemented; physical compression ratio, CPU cost, COW/compaction, and broader supported-host behavior remain acceptance work.
- **v0.22:** browser/native/VS Code notification flows are covered by repository/CI behavior tests; real desktop integration remains platform-dependent.
- **v0.23:** structured logging and redaction are implemented across maintained executables; downstream log collection remains deployment-specific.
- **v0.24:** privileged storage helper lifecycle and ordinary-user `haco create` / `exec` / `delete` / `run` are accepted on disposable GitHub-hosted Ubuntu 26.04 with real Incus and managed Btrfs; this does not prove every supported Host configuration.

## Rule of thumb

> **If a capability is durable enough that we would mention it when describing what Hacocoon can do, taking the next pre-1.0 minor number is fine.**
