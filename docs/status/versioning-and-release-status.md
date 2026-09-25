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

The private Windows review descendant cleanup fix remains within v0.68; it repairs
the existing startup-coordination contract. See [ADR 0110](../adr/0110-private-windows-process-ownership.md).
No tag or release is created by this fix.

The #708 integration with main's responsibility layout remains within v0.68.
It retains the existing named-builder, inventory and Windows startup-coordination
work and reuses main's stream close-completion fix. See the
[candidate-bound validation](acceptance-evidence.md#integration-after-the-responsibility-layout-change).
This integration creates no tag or release.

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
| v0.14 | Git Fetch Plugin | historical — legacy CLI implementation retired |
| v0.15 | OCI Seed Recommendation | historical — Seed implementation removed |
| v0.16 | OCI Image Deletion | historical — current managed image deletion replaces Seed state |
| v0.17 | OCI Seed Builder & Btrfs/COW | historical — Seed implementation removed |
| v0.18 | Docker Compatibility Plugin | historical — legacy CLI implementation retired |
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
| v0.63 | Ordinary Environment cache collection | ✅ implemented |
| v0.64 | Client TCP Forwarding | ✅ implemented |
| v0.65 | Environment Resolver Selection | ✅ implemented |
| v0.66 | Saved Environment data | ✅ implemented |
| v0.67 | Portable Environment data | ✅ implemented |
| v0.68 | Restore saved work by environment name | ✅ implemented |

The current milestone position is **v0.68**. This declaration and the table mirror YAML.

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

## Workspace membership selection

Within the current v0.67 development checkpoint, stopped forks can select retained
members and add registered repositories. This M2 completion slice creates no tag
or release; native acceptance is recorded separately from implementation.

## Existing worktree input

Within v0.67, the M2 candidate adds explicit independent checkout/linked-worktree
import. This uses current contracts without adding old-version compatibility.
Implementation, installed acceptance and deferred performance remain distinct;
no tag or release is created.


## Retained cache maintenance

The current v0.67 candidate adds all-source history, completed-copy recovery and
reviewed cleanup after producer deletion. This is an M4 usability completion
slice within current contracts, without a tag/release or old-version migration.

## Saved-data deletion diagnostics

Within v0.67, the M5 candidate adds read-only snapshot component inspection and
explicit retry guidance. This does not complete M5, release a package, or establish
underlying storage health. See the [owning contract](../design/environment-snapshots.md#inspect-a-failed-deletion).

## Base archive input

Within v0.67, the M4 candidate adds isolated Base archive import and finite builder
resources, reusing input staging and the existing publication/cleanup lifecycle.
This is a development checkpoint, not a release or complete M4/M5 acceptance.

## Notification session handoff

Within v0.67, the Windows candidate separates process ownership from presentation
readiness and bounds startup independently from predecessor cleanup. This corrects
a daily-use startup race; it does not complete human answer acceptance or release
a new package. See [ADR 0100](../adr/0100-notification-session-readiness.md).

## Env cache emptying

The v0.67 M4 candidate adds reviewed cache emptying with retained-data ownership.
Implementation and real-host acceptance remain distinct; this does not complete
M4/M5, create a tag or publish a release.

## Reclamation diagnostics

Within v0.67, bounded failure stages and bilingual next actions improve the ordinary
Windows reclamation path. This does not complete M5 or publish a release; existing
attached-disk and installed pre-dispatch failures remain distinct acceptance gaps.

## Restored-tree comparison

The v0.67 M5 candidate adds portable file-tree comparison to the existing maintenance
evacuation flow. It is a useful comparison step, not full migration acceptance or a release.

## Reclamation language coverage

Within v0.67, shared English/Japanese result and review presentation closes another
daily-use gap. Protocols, consent and lifecycle are unchanged; no release is published.

## Restore by environment name

v0.68 adds latest-ready snapshot selection by recorded capture time. Canonical
restore still creates independent data. Local regression and one installed
Incus journey passed; this checkpoint does not complete M5 or publish a release.

The catalog lifecycle-lock correction is a v0.68 usability fix, not a new checkpoint or release. Person-dependent login and GUI acceptance is explicitly post-release and does not gate integration of implemented work.

Sequential multi-head fetch is another v0.68 usability correction: it removes a cumulative rejection without changing the per-pack or per-ref authority limits. It does not establish large-repository performance acceptance.

Setup outcome localization is a v0.68 daily-use correction with no checkpoint
advance or release. Human login and notification/editor decisions remain
post-release acceptance, not a gate for merging implemented work with green CI.

Main e4d99700 (#699) integrates the retained-data, Git and lifecycle follow-ups within v0.68 after all five exact-head workflows passed. This is main integration, not publication or completed person-dependent acceptance.

Network and configuration result guidance share the v0.68 daily-use checkpoint. Localization does
not change connection authority, create a new checkpoint or publish a release.

The Git streaming candidate remains a v0.68 usability correction. Removing the
32 MiB pack restriction does not establish giant-repository performance or publish
a release. See [ADR 0106](../adr/0106-streaming-git-packs.md).

Main f225e5c1 (#701) integrates setup/network/configuration guidance and the
forwarding fixture correction after all five exact-head workflows passed.
This remains v0.68; person-dependent post-release acceptance is not a main gate.

Named current-data selection and aggregate restored-tree review are a v0.68 M5
maintenance follow-up ([#703](https://github.com/SLktEx/Hacocoon/issues/703)).
The development helper reuses existing comparison contracts. Local success does
not complete actual inventory or person-dependent acceptance; no release is published.

Optional named Base builders are a v0.68 usability correction: ordinary network
settings can target one chosen build name while ownership remains fresh. This does
not advance a checkpoint or establish actual Packer completion or publication.

M2 local Git diagnosis/explicit repair is a v0.68 usability correction. It reuses existing broker ownership and does not advance the checkpoint, grant push permission or publish a release.

The M1 follow-up remains a v0.68 daily-use correction. Bilingual auxiliary-client guidance and successful help-only execution do not advance the checkpoint or publish a release. Human desktop acceptance and full CLI localization remain separate.
