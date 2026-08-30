# Versioning and release status

> **Authoritative milestone numbering · Updated 2026-08-30**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

Use this document for **which version number belongs to which development checkpoint**. Use [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

> **Minor milestones are lightweight pre-1.0 progress checkpoints, not completeness gates.**

1. A coherent product or implementation step may consume the next `v0.N` milestone even when follow-up slices, hardening, or real-host acceptance remain.
2. A partial earlier milestone does not block later milestones. Version order is chronology, not a claim that every previous gate is complete.
3. Granularity is intentionally pragmatic. Closely related work may share a milestone, while a substantial follow-up may take the next minor number.
4. Security/hardening, bug fixes, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only work normally do not consume a product version by themselves, but they may ship inside the current checkpoint.
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

The current milestone position is **v0.21**. Earlier partial milestones remain visible as acceptance/work items but do not prevent later development checkpoints from advancing.

v0.7 keeps its number because its provider-neutral routing seam remains useful. Concrete EC2/AWS/EBS code is absent from the active tree and **cloud implementation is currently deferred**.

**Local Registry infrastructure is deferred and unversioned.** It is not required by normal pulls or Seed construction and does not reserve a roadmap milestone.

## Specification map

Document addresses are semantic and do not change when milestone assignments change.

| Gate | Design document |
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
| Optional Local OCI Registry | [`../OPTIONAL_LOCAL_OCI_REGISTRY.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.md) |

## Acceptance watch list

- **v0.7:** cloud implementation is deferred; acceptance will be redefined when a concrete cloud adapter returns.
- **v0.8:** real Windows/WSL + Incus + client acceptance remains host-dependent.
- **v0.9/v0.10:** real Agent Host/AHP routing remains host-dependent.
- **v0.11/v0.12:** real Base/image and resource-enforcement acceptance remains host-dependent.
- **v0.13:** real supported-Incus network/profile/ACL acceptance remains host-dependent.
- **v0.14:** brokered fetch is implemented; real private-repository combinations remain acceptance-sensitive.
- **v0.15/v0.16:** OCI plugin recommendation/deletion behavior is implemented.
- **v0.17:** repository build/publish plus explicit pin/re-enable, conservative old-revision GC, interrupted-builder recovery, deletion-race protection, and managed-Environment harvest are implemented. Authenticated/private-registry combinations, physical Btrfs COW measurement, broader real-host failure injection, and supported-host acceptance remain.
- **v0.18:** repository lifecycle/CLI integration is implemented; real Base + Incus/systemd socket-activation acceptance remains host-dependent.
- **v0.19:** hostname-aware proxy authorization/enforcement is implemented in the repository; real supported-Incus bridge/nftables/dnsmasq acceptance remains host-dependent.
- **v0.20:** Hacocoon-owned Incus rootfs paths select the lazy managed sparse-raw Btrfs pool; physical COW/compaction measurements and broader supported-host acceptance remain host-dependent.
- **v0.21:** managed Btrfs mounts use `compress=zstd:3`, remount non-compliant managed mounts, reject `compress-force` as the desired state, and avoid automatic recompression that could damage reflink/COW sharing. Real compression ratio, CPU cost, and supported-host behavior remain host-dependent.

## Rule of thumb

> **When a meaningful chunk of product progress lands, taking the next minor number is fine. Do not wait for every acceptance item to close before advancing.**
