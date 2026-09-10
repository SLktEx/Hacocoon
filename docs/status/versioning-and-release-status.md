# Versioning and release status

The v0.57 OCI image cleanup checkpoint is partial. Detached nerdctl Store list/delete through production composition and the bare controller/CLI passed real Incus/Btrfs acceptance at bd1c9a5. Full installed-controller acceptance, detached Docker and candidate-selected GC remain incomplete. See [image operations](../design/oci-image-deletion.md#controllercli-acceptance).

Source repositories and whole OCI Stores support reviewed deletion through existing registry/reservation records and Incus volume/device operations. Workspace Git references, queued request identities and retained snapshots remain protected. Reclamation and export/migration remain separate unfinished work.

Environment copy adds a stopped-source, independent-data convenience flow using existing Incus COW and canonical creation. No schema change or automatic backup is added. See [the copy contract](../design/environment-copy.md).

The v0.50 checkpoint adds public snapshot restore, combining independent Workspace/OCI copies with
canonical saved-rootfs creation and start. Existing Env names are refused, and
failed cleanup retains exact ownership evidence. Schema 13 is unchanged. No Base
filesystem, automatic backup or full runtime recovery is added. See the
[snapshot contract](../design/environment-snapshots.md). In-place replacement,
restored SSH acceptance and live OCI consistency remain incomplete.

Fresh Host setup now binds the owned OCI area automatically; existing data migration and runtime acceptance remain partial.
Preceding checkpoint v0.39 adds the Windows notification review adapter and per-distribution registration. Native history/protocol/stale-refusal checks passed locally; fresh notification decisions and Linux activation remain incomplete. See [implementation status](../IMPLEMENTATION_STATUS.md).


The partial approval slice now connects ordinary Git pending/approve/deny to
reusable saved scope and durable receipts. Notifications, broader config
management remain incomplete. Installed `eb16300b6700` passed dedicated GitHub
push, saved ask reuse and denial acceptance; other choices retain repository-only
coverage. Current Windows GHA failed at the editor wait, after SSH passed. See
[ADR 0026](../adr/0026-reusable-git-approval-scope.md).

Preceding checkpoint v0.37 adds approval configuration inspection and revision-bound
editing through the same Policy and writer as saved approvals. Installed acceptance
is pending.

Preceding checkpoint v0.36 adds automatic Environment DNS configuration in installed
Standard mode and safe restoration of absent stopped-Environment source guards.
Lookup and connection permissions remain separate. Repository regression coverage
exists; the new Windows DNS fixture and real reboot/VPN acceptance are pending.

The preceding v0.33–v0.35 desktop SSH, Host recipe and temporary-execution slices
passed all four GHA workflows at `b6c428d`. The DNS relay foundation at `3c3c101`
also passed all four workflows. Local VS Code 1.136.1 Remote-SSH acceptance passed
on the older `8752431` installation after explicit permission; temporary rules
and the test connection were removed. See [implementation status](../IMPLEMENTATION_STATUS.md)
for the exact scope and the initial resume failure.

Preceding checkpoint v0.32 adds automatic default Store initialization, Workspace
association/reuse, source-only publication state and the optional `--no-oci`.
Ready-source copying passed local component and synthetic real-Btrfs acceptance.
The Host image producer and Docker image/runtime acceptance remain incomplete,
so the entire B4 flow is still partial. v0.31 and the SSH public-key addition
passed all four PR #482 GHA workflows at `f8517ba`.


Preceding checkpoint v0.31 adds retained Environment resume through
`haco env start <name>`. Local test/race and an independent real Incus/WSL
resume fixture and installed-product GHA at `f8517ba` passed. Automatic SSH setup
remains pending. This does not complete roadmap C/E. The preceding v0.30 Store copy
slice and its B4 follow-ups remain described below.


Preceding checkpoint v0.30 adds independent offline persistent Store copies through
`haco plugin oci store create <target> --from <source>`. Repository and local
real-Incus synthetic-data COW acceptance cover this slice; full trusted Host OCI
image delivery, runtime acceptance and interrupted-copy recovery remain partial.
The preceding v0.29 native WSL/Persistent Store/Windows OpenSSH acceptance remains
bound to `c86c43e`. `switch-base` stays disabled and on hold. See
[implementation status](../IMPLEMENTATION_STATUS.md) for evidence and limits.


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

The Incus startup PID guard is a v0.28 maintenance fix. It prevents cross-namespace
replay of network/proxy process records without changing milestone numbering.
See [implementation status](../IMPLEMENTATION_STATUS.md#incus-startup-pid-protection)
for validation and installation limits.

Historical v0.28 acceptance: the candidate added trusted WSL Windows interop, repository collections,
Workspace-preserving Base switching, optional one-way OCI distribution,
OpenSSH configuration generation and readable Environment status. B1–B6 have
local packaged/manual acceptance. Docker and nerdctl distribution, independent
guest start/change/stop, B5/B6 and the A regression passed on packaged `029ff08`.
That completed the earlier local second-stage request; it was not a published
release or acceptance of a broader platform/image matrix.

The v0.27 candidate implements the managed repository WSL workflow: independent
Workspace copies, standard SSH, ordinary Git helper fetch/pull and fixed-content
push approval, followed by graceful stop retaining work. Local real-Git
regressions pass; the branch-built Windows package completed the local A1–A6
journey on `7a4d122`, including approved remote push and retained shutdown.
Manual setup remains; fresh installation and a wider host matrix are unverified
for this candidate.
See [implementation status](../IMPLEMENTATION_STATUS.md) for exact evidence.

Controller-backed setup, trusted networking, controller-owned Standard proxy and read-only configured/live storage diagnostics are implemented within the current checkpoint. Packaged acceptance on `c749ff9033b33c3526e108f60ce2009638075152` passed Windows, Ubuntu and all four Incus jobs, plus local cached BAT application/rerun, ordinary/cold entry, six readiness checks and retained trusted-host data.

The requested WSL M0–M1 scope is **implemented and accepted**: installed Environment allowed-proxy/denied-direct acceptance passed for candidate `81c0d16` (identical-tree PR merge `9049df3`). Registration stop/continuation and the fresh Windows package gate passed on `4df465a`. Actual Windows OS reboot and further continuation work are outside the latest requested scope; do not add broad acceptance matrices or repeat successful local checks without a concrete change or failure. Cross-namespace Incus startup PID replay is addressed by ADR 0013; remaining upstream process-lifecycle limits are recorded separately. [Implementation status](../IMPLEMENTATION_STATUS.md) owns the commit-bound evidence, package identity and acceptance limits.

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
| v0.27 | Managed Repository WSL Workflow | ✅ implemented |
| v0.28 | Multi-repository Development and Optional OCI Distribution | ✅ implemented |
| v0.29 | Persistent OCI Resources and Native Windows Access | ✅ implemented |
| v0.30 | Independent Persistent Store Copies | ✅ implemented |
| v0.31 | Retained Environment Resume | ✅ implemented |
| v0.32 | Automatic Workspace Store Initialization | ✅ implemented |
| v0.33 | Desktop SSH Setup | ✅ implemented |
| v0.34 | Host Setup Recipes | implemented; installed Windows save/replay/update/clear passed at bcc1baf |
| v0.35 | Temporary Execution | implemented; product run and cancellation cleanup passed on real Incus at 4adfe19 |
| v0.36 | Environment Name Resolution | ✅ implemented |
| v0.37 | Approval Configuration Editing | ✅ implemented |
| v0.38 | Pending Approval Review | ✅ implemented |
| v0.39 | Windows Notification Review | ✅ implemented |
| v0.40 | Host OCI Area Copy Boundary | partial |
| v0.41 | Interactive Environment Selection | ✅ implemented |
| v0.42 | Completed OCI Copy Recovery | ✅ implemented |
| v0.43 | Approved AWS S3 Listing | ✅ implemented |
| v0.44 | Verified AWS Object Downloads | ✅ implemented |
| v0.45 | Guest AWS Request Boundary | ✅ implemented |
| v0.46 | Snapshot Workspace and OCI storage | ✅ implemented |
| v0.47 | Automatic Base retention | ✅ implemented |
| v0.48 | Retained Base snapshot capture | ✅ implemented |
| v0.49 | Snapshot restore staging | ✅ implemented |
| v0.50 | Public Snapshot Restore | ✅ implemented |
| v0.51 | Environment Copy | ✅ implemented |
| v0.52 | Base Builder | ✅ implemented |
| v0.53 | Workspace Cleanup | ✅ implemented |
| v0.54 | Base Image Cleanup | ✅ implemented |
| v0.55 | OCI Store Cleanup | ✅ implemented |
| v0.56 | Source Repository Cleanup | ✅ implemented |
| v0.57 | OCI Image Cleanup | partial |

The current milestone position is **v0.57**. This declaration and the Version/Gate columns above are mirrors of `checkpoints.yaml`; the status column remains human-maintained. Earlier partial milestones remain visible as acceptance/work items but do not prevent later development checkpoints from advancing.

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

The C4 Workspace recipe slice extends `haco setup` with an optional Environment
target. Implementation and focused race coverage exist; installed acceptance
is pending, so the C4 roadmap item remains partial.

The current partial approval checkpoint now includes creation-specific Environment identity and Policy binding. This is repository implementation evidence; ordinary Git saved choices and network/provider acceptance remain separate work.

Windows acceptance at d4aef8d now covers the basic C4 recipe cycle, C5 HTTP/Edge preview and C6 Environment doctor prerequisites, alongside VS Code. C4 recreation/cancellation, default-browser launch and VPN/NRPT remain separate acceptance work. The partial approval checkpoint additionally resolves named requests to catalog identity without extra CLI arguments.
