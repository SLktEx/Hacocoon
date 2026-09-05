# Versioning and release status

> **Human-readable checkpoint policy/status view · Updated 2026-08-31**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

[`checkpoints.yaml`](checkpoints.yaml) is the machine-readable source of truth for **checkpoint numbering, the current checkpoint, and Gate identity**. This document explains the numbering policy and carries human-maintained implementation/acceptance status. Use [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

> **Minor milestones are lightweight pre-1.0 progress checkpoints, not completeness gates.**

1. A coherent product, implementation, operator-experience, observability, or acceptance step may consume the next `v0.N` milestone even when follow-up slices, hardening, or real-host acceptance remain.
2. A partial earlier milestone does not block later milestones. Version order is chronology, not a claim that every previous gate is complete.
3. Granularity is intentionally pragmatic and intentionally aggressive during pre-1.0 development. Closely related work may share a milestone, while a substantial follow-up may take the next minor number.
4. Security/hardening, bug fixes, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only changes do not automatically consume a milestone, but they may do so when they create a meaningful support, operability, or acceptance checkpoint.
5. Milestone changes go through `tools/bump-milestone`, which updates `checkpoints.yaml`, this human-readable table/current declaration, the Japanese mirror, `../IMPLEMENTATION_STATUS.md`, and generated build identity together.
6. Design-only specifications may reserve future numbers but remain **planned** until implementation lands.
7. Historical commit messages, PR titles, candidate branches, old document addresses, and superseded numbering are not authoritative.
8. Tags/releases are separate from roadmap milestone numbering.

## Current checkpoint status

The dedicated trusted-host networking correction stays within the current checkpoint. Ownership, profile-independent fresh creation, scoped Docker forwarding and data-preserving current-host migration are implemented; their final packaged acceptance is tracked in implementation status.

Observed Windows acceptance on 2026-09-06: `57b6ee2` completed fresh cached BAT installation, but its normal shell exit was later found to hang. Packaged `8a44f17` corrected cold entry and normal process exit; final `3f67845` passed cached BAT application, same-version rerun, cold entry, controller communication, actual exit 0 and trusted-host file/identity/account/sudo-policy retention. Fresh creation of the final candidate was not repeated. Physical Host and trusted-host HTTPS returned 200 on `3f67845`; earlier trusted-host attempts timed out under Docker FORWARD DROP, whereas the later read-only rules showed ACCEPT. No manual firewall repair was applied, and startup-order coexistence remains unverified. Environment proxy enforcement, SSH and Workspace work retention are separate pending acceptance. These observations predate the dedicated-network correction described below.

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
| v0.20 | Managed Btrfs Rootfs Storage | ✅ Incus-owned loop-backed Btrfs pool and Hacocoon rootfs routing implemented; broader physical COW/compaction acceptance remains host-dependent |
| v0.21 | Managed Btrfs Transparent Compression | ✅ default Incus pool creation requests `compress=zstd:3` without `compress-force`; real compression/performance acceptance remains host-dependent |
| v0.22 | Interaction Notification Clients | ✅ browser, native OS, and VS Code notification clients implemented with replay/dedup behavior tests |
| v0.23 | Real Incus E2E Acceptance | ✅ standalone Incus substrate and Hacocoon Core lifecycle are exercised on GitHub-hosted Ubuntu 26.04 with phased gating |
| v0.24 | Structured Logging | ✅ shared `log/slog` foundation, operation context, sanitized DEBUG tracing, and secret redaction implemented across maintained executables |
| v0.25 | Incus-owned Btrfs Storage Acceptance | ✅ ordinary-user real Incus/Btrfs CLI acceptance verifies the Incus-owned pool lifecycle and policy |
| v0.26 | Trusted `haco-host` & Default WSL Entry | ✅ persistent trusted logical Host lifecycle, ownership/collision checks, managed-storage placement, default WSL entry, recovery path, and real Incus acceptance implemented |

The current milestone position is **v0.26**. This declaration and the Version/Gate columns above are mirrors of `checkpoints.yaml`; the status column remains human-maintained. Earlier partial milestones remain visible as acceptance/work items but do not prevent later development checkpoints from advancing.

v0.7 keeps its number because its provider-neutral routing seam remains useful. Concrete EC2/AWS/EBS code is absent from the active tree and **cloud implementation is currently deferred**.

**Local Registry infrastructure is deferred and unversioned.** It is not required by normal pulls or Seed construction and does not reserve a roadmap milestone.

## Specification map

Document addresses are semantic and do not change when milestone assignments change.

| Gate | Design/reference document |
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
| v0.22 Interaction Notification Clients | [`../INTERACTION_EVENTS.md`](../INTERACTION_EVENTS.md) |
| v0.24 Structured Logging | [`../reference/logging.md`](../reference/logging.md) |
| v0.25 Incus-owned Btrfs Storage Acceptance | [`../design/btrfs-storage-layout.md`](../design/btrfs-storage-layout.md) |
| v0.26 Trusted `haco-host` & Default WSL Entry | [`../design/trusted-host.md`](../design/trusted-host.md) |
| Optional Local OCI Registry | [`../OPTIONAL_LOCAL_OCI_REGISTRY.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.md) |

v0.23 is an acceptance checkpoint rather than a new architecture contract. Its executable specification lives in the GitHub Actions/CI harness and its support boundary is summarized in `IMPLEMENTATION_STATUS.md`.

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
- **v0.20:** Hacocoon-owned Incus rootfs paths select the lazy `haco-local-default` Incus-owned loop-backed Btrfs pool; physical COW/compaction measurements and broader supported-host acceptance remain host-dependent.
- **v0.21:** creation of the default Incus-owned Btrfs pool requests `compress=zstd:3`, does not request `compress-force`, and leaves `autodefrag` disabled. Real compression ratio, CPU cost, and supported-host behavior remain host-dependent.
- **v0.22:** browser/native/VS Code notification delivery and replay/dedup behavior are repository-tested; desktop/session-specific delivery still depends on the actual client environment.
- **v0.23:** GitHub-hosted Ubuntu 26.04 proves standalone Incus system-container behavior before Core lifecycle E2E; this narrows the CI support gap but does not prove every supported Host/WSL configuration.
- **v0.24:** maintained executables share structured logging and redaction behavior; logging policy remains defense in depth and does not make unsafe call-site data safe to emit.
- **v0.25:** ordinary-user `haco create`/`exec`/`delete`/`run` are exercised against real Incus; CI verifies the Incus-owned sparse backing file, loop attachment, Btrfs mount, zstd policy, pool reuse, and cleanup. Broader physical-storage and Windows/WSL acceptance remains.
- **v0.26:** trusted-host creation, exact ownership/collision handling, idempotent ensure, stopped-state recovery, managed-storage placement, and raw control-socket non-exposure are covered by real Incus acceptance. Real Windows/WSL interactive-login behavior remains host-dependent, and broader Git/OCI/credential/control-channel migration remains follow-up work.

## Rule of thumb

> **When a meaningful chunk of product, operator, observability, or acceptance progress lands, taking the next minor number is fine. During pre-1.0 development, prefer visible checkpoints over conserving version numbers.**
