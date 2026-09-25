# Architecture and roadmap

Hacocoon is a **Secure Workspace Runtime**. Developer tools and agents work inside
isolated Environments; Host and external authority remain behind explicit boundaries.
This page owns unfinished work. [Implementation status](../IMPLEMENTATION_STATUS.md)
owns current features, and [acceptance evidence](acceptance-evidence.md) owns exact
passes, failures and skips.

## Current scope and integration

The overall roadmap remains M0–M5 on `main`. The current work unit is **M1/M2
integration**. PR #727 includes both #726's daily-use improvements and M2 Git
recovery, including their Windows E2E fixes. It is the combined main candidate;
#726 remains a reviewable source branch until integration succeeds. Do not start
M3–M5. Local checks are primary; main merge requires all five exact-head workflows.
Tags/releases and person-dependent acceptance remain separate.

M2 and M3 real-use acceptance is assigned to the user: authenticated Git and
GUI answers, plus VPN/DNS and other non-ordinary-network combinations. These
remain unperformed until reported; they do not hold implementation delivery.

User login, human notification/VS Code answers and other person-dependent checks
are post-release acceptance items, not conditions for merging implemented work
into main. Keep their unperformed status explicit without holding implementation
delivery for them. This does not turn a known CI failure into a pass.

Main `bfa19ecb` / [#694](https://github.com/SLktEx/Hacocoon/pull/694) includes
the earlier M0–M5 implementation, retained-data and lifecycle corrections from
#687–#699, shared bilingual setup/network/configuration guidance from
[#701](https://github.com/SLktEx/Hacocoon/pull/701) (including #700), and bounded
Git pack streaming, responsibility-based directories and removal of the old CLI.
Each merged checkpoint passed all five exact-head workflows.
Earlier HTTP503, attached-disk, TCP and #700 rerun failures remain in
[acceptance evidence](acceptance-evidence.md); later success does not rewrite them.

This checkout also implements named current-data selection in
[#704](https://github.com/SLktEx/Hacocoon/pull/704) and named Base builders in
[#705](https://github.com/SLktEx/Hacocoon/pull/705). Their PRs track integration;
local validation and ordinary installer-package generation are recorded in the
acceptance evidence. No release has been published for these changes.

## M0–M5 remaining work

Main `bfa19ecb` / #694 now owns the responsibility layout and retires the old CLI.
#708 is integrated into that layout on its development branch, retaining #704–#707's selection,
named-builder, observation and binding-inventory work. Its previous exact head
`7e5971b5` passed installed Windows reclamation and retained-data restoration;
its separate race failure is addressed by main's existing close-completion fix.
See [the distinct results](acceptance-evidence.md#integration-after-the-responsibility-layout-change).
The installed local candidate `d8ec1374` passed doctor, retained Workspace reuse
and independent snapshot restoration. Its Incus product jobs passed, but the
evidence job timed out waiting for terminal API metadata. The follow-up retains
that failure and extends only bounded read-only observation; main integration
still requires all five workflows to succeed. Long help names also receive an
explicit separator in both languages after the defect was observed locally.
M2/M3 person-dependent acceptance remains assigned to the user.

The user has replaced the local WSL and selected only Hacocoon development Git
data for retention. Five source trees were archived and restoration-checked on
Windows. Restoring the deliberately excluded old installation/application data
is no longer a prerequisite. Ordinary product transfer/retention, guest-visible
ownership and authenticated restored use retain their separate acceptance scopes.

The earlier main checkpoint [#702](https://github.com/SLktEx/Hacocoon/pull/702) integrated
the Git streaming correction after all five exact-head workflows passed. The
single-pack functional limit is resolved within the documented 16 GiB budget;
representative huge-repository performance remains deferred. Installed acceptance
of the new large-pack path is separate from the successful ordinary Git CI paths.

| Stage and useful outcome | Remaining implementation or acceptance |
|---|---|
| M0 — use existing improvements together | Main integrates the earlier implementation through #702. The same-version installation, ordinary SSH/editor/forwarding, stop/resume and retained-data cycle passed packaged Windows and Incus CI. Broader configurations remain separate. |
| M1 — understandable everyday use | Vertical help, language handoff, repeated-failure grouping, installer results, bilingual reclamation and setup/network/configuration results are on main. Other command families remain in the language contract. Human GUI/layout checks are post-release. The dedicated managed-user systemd-session warning remains a separate observation. |
| M2 — independent multi-repository work and reviewed Git | Independent multi-repository forks, checkout/linked-worktree input, all-head fetch, reviewed new/fast-forward push and result reconciliation are on main. Incremental history and sequential bounded fetch are integrated. Authenticated development and fresh notification/VS Code answers remain post-release acceptance. Main push approval stays independent of clone/fetch. |
| M3 — permitted communication outside ordinary networks | TCP/UDP, loopback forwarding, interactive temporary execution and persistent host/backend/disabled DNS selection are implemented. Supported-baseline DNS/Policy and ordinary Windows forwarding have scoped evidence. Actual VPN/NRPT/restart combinations remain unperformed; name discovery never grants connection authority. |
| M4 — reuse Base, cache and OCI data | Real Packer/HCL2/external shell and Base import are implemented; #705 adds a named build target for scoped configuration. The installed attempt failed during dependency downloads; build/publication/reuse remains unperformed. Creation-time cache enrollment, independent generation reuse, collection/history/recovery/clear and Env-local emptying exist. Late enrollment and unknown-copy cancellation are follow-ups. Broader OCI acceptance and representative large-repository measurements are deferred. Main's streaming implementation removes the 32 MiB single-pack restriction with local real-Git evidence; installed large-pack acceptance remains pending. |
| M5 — understand retained data and resume development elsewhere | Inventory, component deletion diagnosis, reviewed retained-object cleanup, export/import, restored-tree comparison and latest restore by source name are on main. #704 adds named selection and aggregate comparison. The selected development Git trees are independently retained on Windows. The new WSL passed ordinary snapshot/restore, Windows-file export/import, guest-owner checks and post-deletion reuse for a small Workspace. Local public reclaim recovered 1,742,733,312 bytes with retained-data verification afterward. Populated OCI applications and authenticated resumed development remain separate; earlier CI/local failures remain recorded. |

[CLI language scope](../reference/cli-language.md),
[Workspace input](../design/workspace-input.md),
[Packer builds](../design/packer-base-builds.md),
[cache generations](../design/cache-generations.md), and
[data evacuation](../guides/data-evacuation.md) define the actual supported operations.
Do not expand a pending acceptance item into an unrelated compatibility project.

## Acceptance priorities and limits

The M1 follow-up starts from main `68c539c1` (#722), which already includes
Windows private-review descendant cleanup (#721). It adds shared bilingual
pending-approval, preview and experimental-editor guidance, and repairs successful
help-only execution in `haco-vscode`. See the [language contract](../reference/cli-language.md).
Installer completion and vertical help reuse #573/#575; no replacement installer
or SSH ownership implementation is needed. Full localization remains partial
(#577). Human notification/editor responses and packaged layout/language acceptance
remain post-release checks; neither this implementation nor a local test pass
marks them performed. Next M1 acceptance is the ordinary installed Japanese journey:
install, help, Workspace/Environment, VS Code, stop and resume, including resize
and a failed SSH connection. M2–M5 are outside this work unit.

M1's ordinary SSH/editor selection and result guidance now use the shared Japanese/
English catalog in this checkout. Invalid preview options explain the correction
before preparing a Workspace. Local CLI/real-PTY and maintained repository checks
passed; main integration and installed desktop acceptance remain separate.

M5's named current-data selection and aggregate comparison helper is implemented
in this checkout for [#703](https://github.com/SLktEx/Hacocoon/issues/703).
It reuses the existing portable manifest comparer and preserves partial/unreviewed
outcomes. The operator's actual selection was subsequently narrowed to five
development Git trees, independently retained and restoration-checked on Windows.
Normal snapshot/export/import and post-deletion reuse also passed for one small
managed Workspace, including its guest owner. Populated OCI applications and
authenticated resumed development remain separate acceptance items.
No old-version reconstruction or automatic deletion is added.

The current inventory now projects saved Git bindings, including collection
members, without exposing remote URLs or granting connection authority. Read-only
use on the dedicated WSL resolved the previously unprojected `bindings` directory
(two repository files and one binding). Actual selection and restored use remain
separate; this does not turn saved associations into current authority.

The installed Packer attempt failed during ordinary dependency downloads with
apt exit 100, before Packer/HCL execution. No human notification answer was
confirmed. Its three temporary named-builder-only require-approval rules were
removed afterward, restoring the original default deny and zero rules. The
earlier all-Environment proposal was refused and never applied. Before another
attempt, confirm the user's normal notification route and review only the needed
destinations. Build/publication/reuse remains unperformed; no test allow rule or
automated GUI answer substitutes for that acceptance.

The implemented `--builder <env>` option allows naming one build target for
ordinary administrator rules, without adopting an existing Env or changing Policy.
This removes the need to know a random name in advance. The rejected broad rule
has not been applied; the installed scoped configuration does not itself approve
downloads or prove build/reuse. It is a usability improvement
within M4, not proof that dependency downloads have succeeded.

Reclamation succeeded with measured allocation recovery in Windows CI, including
retained Workspace/OCI verification. The dedicated local start previously failed
at enrollment while a direct Windows observation passed; a later Japanese read-only
status succeeded after normal installation. That does not explain the earlier
failure or prove a new start. An attempted global WSL restart was stopped by the
pre-action guard after an unrelated running distribution was observed; no global
shutdown occurred. Later, restarting only the dedicated WSL preserved the failure.
Later #704/#705 Windows runs again refused compaction because the disk remained
attached after the bounded wait. Preserve those failures alongside earlier recovery
success. Windows-only process-count observations now support distinguishing the
shutdown/detachment interval without restarting WSL, changing the product timeout
or weakening refusal. The first installed result (#706, `38aeae56`) showed WSL
processes disappearing and later returning, but still failed with `compact_attached`.
Fixed parent-chain categories now narrow the next investigation; the restart source
and detachment cause remain pending under #381. Counts never authorize compaction.
The `c3a2cc86` retry also failed: the returning launcher had an unclassified parent.
The category set now includes Hacocoon's native review/client helpers and Windows
WSL host/relay/services; the earlier category set could not exclude those sources.
The #707 retry identified notification ancestry when WSL restarted. The current
candidate adds shared startup/reclamation coordination (ADR 0108), with native
component evidence. #708 retained host processes without a new observed start;
that failure is not yet explained. Installed acceptance of the coordination fix
remains pending. Local follow-up also encountered zero free bytes on C: and WSL
CreateInstance/E_FAIL; only generated test artifacts were removed.
Read-only comparisons isolate enrollment visibility to the init interop route;
the underlying Windows cause remains unproven. Human notification/VS Code answers remain unperformed.

Current data was archived and restored to another WSL with content, raw Host owner,
link and regular-file/directory-attribute comparison. This is a small real-data
result, not guest-idmap acceptance, all-data restoration or a giant-repository claim.
Authentication, external networks and visible desktop acceptance must be recorded
as performed, failed or unperformed independently of repository tests.

Large-repository performance, additional strict validation, broader OCI/runtime
compatibility are deferred until ordinary use works. Functional Git transfer
limits have been addressed on main: incremental transfer avoids resending known
history for fetch and existing-target push. New-branch push also reuses one
advertised ancestor through an independent exact-ref read. Streaming replaces the 32 MiB new-pack
restriction with bounded frames and a finite 16 GiB per-pack transfer limit,
without whole-pack JSON/base64 buffering. It preserves separate push approval;
local 40 MiB real-Git acceptance is functional evidence, not completed
giant-repository performance acceptance. Installed Incus validation is pending. Measure representative
large repositories before claiming the intended capacity/speed benefit; small
fixtures do not prove it. LFS, submodules, force/delete/multi-ref push are not
additional M2 completion conditions.

## Architecture and retained data

Core owns Workspace identity/leases, Environment lifecycle, Execution, resource
budgets, Policy/Approval/Audit and narrow client/capability contracts. Standard
supplies replaceable implementations; optional integrations belong to Plugins.
Provider and client details stay behind their boundaries. Hacocoon does not own
AI orchestration, model routing, task graphs, code review or merge decisions.

The Physical Host keeps Incus/platform authority. Trusted `haco-host` is management
infrastructure inside the TCB. Ordinary Environments never receive management
sockets, protected state or reusable Host credentials. Incus owns its backing
storage; Hacocoon records associations and exact lifecycle ownership.

Environment deletion retains contracted Workspace and OCI data. Cleanup releases
ownership only after positive absence, including guard cleanup. Unknown copies
and ambiguous cleanup retain ownership. Cache collection publishes complete
numbered generations with comparison against the starting generation; it does not
merge files or share writable sources. Snapshot rootfs/data is independent; Base
is provenance. Preserve the [security boundary](../security/security-architecture.md)
and [logging contract](../reference/logging.md) in every stage.

## Deferred and excluded work

Old-version compatibility, Seed migration readers, old-WSL reconstruction and
replacement are outside the current request. Seed runtime/build/sampling retirement
is integrated; code retirement does not authorize deleting existing application data.
`switch-base` has no planned return: ordinary new-Env creation uses retained data.

Management Web UI, another PC, cloud/VM/other infrastructure, DB volumes, mandatory
local registry, live migration, simultaneous writable Store sharing, Packer AMI/QEMU
and optional real AWS acceptance are future scope. They are not gates for M0–M5.
Same-PC Windows/WSL remains first. Checkpoint numbering and history stay in
[versioning and release status](versioning-and-release-status.md).

## M2 current implementation unit

Based on main `68c539c1`, this checkout adds Git broker diagnosis to Environment
doctor and explicit local repair through the existing connection API. User-facing
`git connect` is removed. Reuse the earlier M2 implementations; #617 tracks this
remaining daily-use recovery path. The [owning contract](../design/git-and-github-capability.md#local-git-diagnosis-and-repair)
separates observed wiring, unknown state and repair limits. Authenticated Git and
human Windows/VS Code responses remain user acceptance after release. Large-repo
performance remains deferred. The prior local large-push approval timeout and
milestone-wrapper timeout from M1 are unresolved evidence, not new M2 requirements
or successful results. Next: resolve the M1 Windows attached-disk failure and validate the corrected M2
Windows doctor observer, then merge the combined candidate after required CI and perform
the assigned human acceptance. M3–M5 are outside this unit.

## M1/M2 Windows integration blocker

M1 #726 head `5b447f9c` failed installed reclamation with `compact_attached`;
M2 #727 fixes the separate doctor-observer failure at `038a44c0`. The editor E2E
cleanup defect described in [acceptance evidence](acceptance-evidence.md#editor-fixture-descendant-cleanup)
is now corrected in M1. Next: verify the installed Windows route and merge combined PR #727 only
after all five workflows pass on its final head. Previous disk failures remain unresolved evidence
until their cause is established. No tag/release or next milestone is included.
