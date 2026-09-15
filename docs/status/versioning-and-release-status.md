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
| v0.15 | OCI Seed Recommendation | historical — Seed implementation removed |
| v0.16 | OCI Image Deletion | historical — current managed image deletion replaces Seed state |
| v0.17 | OCI Seed Builder & Btrfs/COW | historical — Seed implementation removed |
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
| v0.58 | Daily CLI Entry and Setup Diagnostics | ✅ implemented |
| v0.59 | Host customization lifecycle and results | ✅ implemented |
| v0.60 | Git branch read and push authority | ✅ implemented |
| v0.61 | Interactive temporary execution | ✅ implemented |
| v0.62 | Packer HCL2 Base builds | ✅ implemented |

The current milestone position is **v0.62**. This declaration and the table mirror YAML.

Concrete cloud implementation is currently deferred. Local Registry infrastructure is deferred and unversioned. Automatic Base filesystem retention (historical v0.47–v0.49) was replaced by the independent saved-rootfs model in [ADR 0040](../adr/0040-incus-first-snapshots.md).

## Integrated development work

The `dev/1.x`, `dev/v2` and `dev/2.x` changes are integrated into main: lifecycle
refactoring, daily setup diagnostics, explicit TCP/UDP connections and path-based
Workspace reopen/data forks. The imported v0.58 checkpoint remains the numbering
authority; integration creates no release or tag. See [current scope](../IMPLEMENTATION_STATUS.md)
and [scoped evidence](acceptance-evidence.md#development-branch-integration).

## Main language integration

The existing v0.59 checkpoint receives a partial integration of #580/#583 daily
CLI language/help work. This reuses a development result and does not consume a
new checkpoint or publish a release. Windows presentation/GUI acceptance remains
separate; see [language scope](../reference/cli-language.md).

The next v0.59 M1 candidate reuses native failure grouping and BAT final results.
Local regression/native component checks are scoped evidence; no checkpoint or
release identity changes. See [acceptance](acceptance-evidence.md#main-notification-installer).

The same v0.59 checkpoint now includes the normalized Host-session language
handoff and read-only Windows UI-language selection. The installer preserves OS
locale; this integration consumes no new checkpoint or release identity.

## Seed retirement in the current development checkpoint

Within v0.59, the M5 candidate removes the executable Seed path and its catalog,
sampling/recommendation and legacy deletion/re-enable state. Current Base and
persistent OCI image operations remain. Old-version compatibility and migration
are excluded by the user's 2026-09-15 scope correction; they do not block M0–M5.
This is code cleanup within the current checkpoint, not a tag or release.

## Git branch workflow integration

v0.60 records all-head reads and explicit new-branch/fast-forward push authority
reused from #585/#587 on main. It does not mark all of M2 complete or publish a
release. GUI, authenticated installed use and large-pack transport remain separate;
see [verification scope](acceptance-evidence.md#main-git-branches).

## Interactive temporary execution

v0.61 records stdin/TTY and streamed output with exact creation-bound cleanup.
It follows the main-targeted Git checkpoint in #663. This is development progress,
not all of M3 or Windows/Incus acceptance and not a release. See
[verification](acceptance-evidence.md#main-interactive-run).
