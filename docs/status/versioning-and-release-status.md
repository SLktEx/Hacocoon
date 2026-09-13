# Versioning and release status

[日本語](versioning-and-release-status.ja.md) | English

Hacocoon is pre-1.0. Checkpoints mark progress; they are separate from compatibility guarantees, published releases and host support. [checkpoints.yaml](checkpoints.yaml) owns numbering, the current value and Gate identity. Consult [implementation status](../IMPLEMENTATION_STATUS.md) and [acceptance evidence](acceptance-evidence.md) for current scope.

## Numbering policy

**Minor milestones are lightweight pre-1.0 progress checkpoints, not completeness gates.**

- A meaningful product, implementation, operator, observability or acceptance slice may consume the next minor.
- Earlier partial work or pending real-host acceptance does not block later checkpoints. Number order records chronology, not completion.
- Fixes, docs, tests, CI and refactors do not automatically advance a checkpoint; judge whether they introduce a meaningful new progress slice.
- Use `tools/bump-milestone v0.N "Gate Name"` to change numbering, updating YAML, paired tables, implementation status and generated build identity together.
- Design-only plans remain planned. Old commits, PR titles and development branches do not override the source.
- Tags/releases are separate from roadmap numbering. Follow the [release checklist](../guides/releasing.md) for publication.

## Checkpoint history

The Version/Gate columns mirror YAML. This is a history of development slices,
not a claim that every old interface remains in the current product CLI.
Current feature scope and remaining work are consolidated in implementation status and the roadmap.

| Version | Gate | Checkpoint state |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | implemented — see current feature limits |
| v0.2 | Workspace Abstraction & Lease | implemented — see current feature limits |
| v0.3 | Client & Interactive Access | implemented — see current feature limits |
| v0.4 | Policy & Capability Foundation | implemented — see current feature limits |
| v0.5 | Git / GitHub Capability | implemented — see current feature limits |
| v0.6 | Agent & Orchestrator Integration | implemented — see current feature limits |
| v0.7 | Remote / Cloud Runtime & External Capabilities | deferred — concrete cloud provider removed; routing seam retained |
| v0.8 | Client Adapters & VS Code Integration | implemented — see current feature limits |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | implemented — see current feature limits |
| v0.10 | VS Code Remote Agent Host Adapter | implemented — see current feature limits |
| v0.11 | Base Images & Custom Environments | implemented — see current feature limits |
| v0.12 | Sandbox Resource Limits | implemented — see current feature limits |
| v0.13 | Managed Sandbox Network | implemented — see current feature limits |
| v0.14 | Git Fetch Plugin | implemented legacy slice — temporary hacoq; not the ordinary Store workflow |
| v0.15 | OCI Seed Recommendation | implemented legacy slice — temporary hacoq; not the ordinary Store workflow |
| v0.16 | OCI Image Deletion | implemented legacy slice — temporary hacoq; not the ordinary Store workflow |
| v0.17 | OCI Seed Builder & Btrfs/COW | partial — implementation and acceptance gaps remain |
| v0.18 | Docker Compatibility Plugin | implemented legacy slice — temporary hacoq; not the ordinary Store workflow |
| v0.19 | Domain-aware Egress Authorization | implemented — see current feature limits |
| v0.20 | Managed Btrfs Rootfs Storage | implemented — see current feature limits |
| v0.21 | Managed Btrfs Transparent Compression | implemented — see current feature limits |
| v0.22 | Interaction Notification Clients | implemented — see current feature limits |
| v0.23 | Real Incus E2E Acceptance | implemented — see current feature limits |
| v0.24 | Structured Logging | implemented — see current feature limits |
| v0.25 | Incus-owned Btrfs Storage Acceptance | implemented — see current feature limits |
| v0.26 | Trusted `haco-host` & Default WSL Entry | implemented — see current feature limits |
| v0.27 | Managed Repository WSL Workflow | implemented — see current feature limits |
| v0.28 | Multi-repository Development and Optional OCI Distribution | historical — replaced by persistent resources / independent saved rootfs |
| v0.29 | Persistent OCI Resources and Native Windows Access | implemented — see current feature limits |
| v0.30 | Independent Persistent Store Copies | implemented — see current feature limits |
| v0.31 | Retained Environment Resume | implemented — see current feature limits |
| v0.32 | Automatic Workspace Store Initialization | implemented — see current feature limits |
| v0.33 | Desktop SSH Setup | implemented — see current feature limits |
| v0.34 | Host Setup Recipes | implemented — see current feature limits |
| v0.35 | Temporary Execution | implemented — see current feature limits |
| v0.36 | Environment Name Resolution | implemented — see current feature limits |
| v0.37 | Approval Configuration Editing | implemented — see current feature limits |
| v0.38 | Pending Approval Review | implemented — see current feature limits |
| v0.39 | Windows Notification Review | implemented — see current feature limits |
| v0.40 | Host OCI Area Copy Boundary | partial — implementation and acceptance gaps remain |
| v0.41 | Interactive Environment Selection | implemented — see current feature limits |
| v0.42 | Completed OCI Copy Recovery | implemented — see current feature limits |
| v0.43 | Approved AWS S3 Listing | implemented — see current feature limits |
| v0.44 | Verified AWS Object Downloads | implemented — see current feature limits |
| v0.45 | Guest AWS Request Boundary | implemented — see current feature limits |
| v0.46 | Snapshot Workspace and OCI storage | implemented — see current feature limits |
| v0.47 | Automatic Base retention | historical — replaced by persistent resources / independent saved rootfs |
| v0.48 | Retained Base snapshot capture | historical — replaced by persistent resources / independent saved rootfs |
| v0.49 | Snapshot restore staging | historical — replaced by persistent resources / independent saved rootfs |
| v0.50 | Public Snapshot Restore | implemented — see current feature limits |
| v0.51 | Environment Copy | implemented — see current feature limits |
| v0.52 | Base Builder | implemented — see current feature limits |
| v0.53 | Workspace Cleanup | implemented — see current feature limits |
| v0.54 | Base Image Cleanup | implemented — see current feature limits |
| v0.55 | OCI Store Cleanup | implemented — see current feature limits |
| v0.56 | Source Repository Cleanup | implemented — see current feature limits |
| v0.57 | OCI Image Cleanup | partial — implementation and acceptance gaps remain |
| v0.58 | Daily CLI Entry and Setup Diagnostics | implemented on development candidate |
| v0.59 | All-branch Git Fetch | implemented on development candidate; native acceptance pending |
| v0.60 | Reviewed Git Branch Creation | implemented on development candidate; native acceptance pending |
| v0.61 | Local GUI Approval Review | implemented on development candidate; installed GUI acceptance pending |
| v0.62 | Temporary Process Streams | implemented on development candidate; native pipe/TTY acceptance pending |
| v0.63 | Durable Git Push Reconciliation | ✅ implemented |
| v0.64 | Notification-contained Approval | ✅ implemented |

The current milestone position is **v0.64**. This declaration and the table mirror YAML.

Concrete cloud implementation is currently deferred. Local Registry infrastructure is deferred and unversioned. Automatic Base filesystem retention (historical v0.47–v0.49) was replaced by the independent saved-rootfs model in [ADR 0040](../adr/0040-incus-first-snapshots.md).

The M0/M1 candidate in PR #583 additionally integrates the existing development
branches and the shared Incus 7.0 LTS installer/doctor contract. It retains the
v0.58 development checkpoint while native acceptance remains partial. This is
not a new tag, main merge, distributed installer or public release.

The separate M2 candidate advances to v0.59 for all-head discovery, per-ref
read authorization and ordinary branch switching. Earlier M1 native gaps do
not block this lightweight development checkpoint. See the Git guide for limits;
this does not claim complete M2, large-pack support or a release.

The v0.60 development slice adds separately reviewed creation of one Git branch
and updates to one exact existing branch. Real Git component and full local test
CI pass; native Git, GUI decisions and large-repository acceptance remain pending.

The v0.61 slice replaces VS Code terminal answers with a local GUI and reuses the existing approval/Policy service through a private bounded session. Windows notification-contained answers remain open. This checkpoint is not a release or completion of M2.

The v0.62 slice adds stdin and optional TTY to temporary execution through the
common creation/cleanup lifecycle. It preserves separate piped outputs, actual
exit codes, input bounds and confirmation of cleanup. Native pipe/terminal gates
remain pending; this is neither M3 completion nor a distributed release.

The v0.64 slice replaces Windows terminal answers with native notification pages, selection controls and COM activation over the existing private review session. It is a development checkpoint, not completion of M2/M3 or a distributed release. Fresh installed answers and visible layout remain pending.
