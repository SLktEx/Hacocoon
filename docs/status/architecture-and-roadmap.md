# Architecture and roadmap

Hacocoon is a **Secure Workspace Runtime**. Developer tools and agents work inside
isolated Environments; Host and external authority remain behind explicit boundaries.
This page owns unfinished work. [Implementation status](../IMPLEMENTATION_STATUS.md)
owns current features, and [acceptance evidence](acceptance-evidence.md) owns exact
passes, failures and skips.

## Current scope and integration

The active roadmap is M0–M5 on `main`, following the user's current instructions.
Usable ordinary flows come first. Local checks are primary; independent development
continues while hosted CI runs. Main merges are authorized only after all five
required workflows succeed for the exact PR head. Tags and releases are separate.

User login, human notification/VS Code answers and other person-dependent checks
are post-release acceptance items, not conditions for merging implemented work
into main. Keep their unperformed status explicit without holding implementation
delivery for them. This does not turn a known CI failure into a pass.

Main `2f421006` (#687) and `ee8bf7fb` (#688) integrated the earlier M0–M5
implementation and reclamation diagnostics. Main `e4d99700` /
[#699](https://github.com/SLktEx/Hacocoon/pull/699) now also contains #689–#693
and #696–#698: restored-tree comparison, bilingual reclamation, SSH failure
classification, restore by source name, incremental/sequential Git transfer,
catalog-scoped lifecycle locking and virtual-disk observation lifetime fixes.

All five required workflows succeeded for exact #699 head `51ba4f24`, including
installed Windows and real Incus/Btrfs. Earlier HTTP503, attached-disk and
unexplained TCP failures remain in [acceptance evidence](acceptance-evidence.md);
later success does not rewrite those receipts. No release was published.

[#700](https://github.com/SLktEx/Hacocoon/pull/700) is the next main-targeted
slice: Host/project setup outcomes and next actions use shared English/Japanese
presentation and vertical help. Local full checks and normal installer-package
generation passed. Four exact-head workflows passed; the first Windows attempt
failed at public reclamation with an attached disk, without starting compaction.
One failed-jobs rerun is pending; retain the failure separately from main #699's
success. [#701](https://github.com/SLktEx/Hacocoon/pull/701) adds network result
guidance and corrects stale cache-maintenance scope descriptions, with local
checks and normal package generation passed. Both remain development work until
their respective main integration.

## M0–M5 remaining work

Main `6cdfe5d0` / [#702](https://github.com/SLktEx/Hacocoon/pull/702) now contains
the Git streaming correction after all five exact-head workflows passed. The
single-pack functional limit is resolved within the documented 16 GiB budget;
representative huge-repository performance remains deferred. #704 is rebased onto
this main and awaits its new exact-head CI; its earlier results are retained separately.

| Stage and useful outcome | Remaining implementation or acceptance |
|---|---|
| M0 — use existing improvements together | Main integrates #687–#688 and #699. The same-version installation, ordinary SSH/editor/forwarding, stop/resume and retained-data cycle passed packaged Windows and Incus CI. Broader configurations remain separate. |
| M1 — understandable everyday use | Vertical help, language handoff, repeated-failure grouping, installer results and bilingual reclamation are on main. #700 completes the setup-result guidance slice; other command families remain in the language contract. Human GUI/layout checks are post-release. The dedicated managed-user systemd-session warning remains a separate observation. |
| M2 — independent multi-repository work and reviewed Git | Independent multi-repository forks, checkout/linked-worktree input, all-head fetch, reviewed new/fast-forward push and result reconciliation are on main. Incremental history and sequential bounded fetch are integrated. Authenticated development and fresh notification/VS Code answers remain post-release acceptance. Main push approval stays independent of clone/fetch. |
| M3 — permitted communication outside ordinary networks | TCP/UDP, loopback forwarding, interactive temporary execution and persistent host/backend/disabled DNS selection are implemented. Supported-baseline DNS/Policy and ordinary Windows forwarding have scoped evidence. Actual VPN/NRPT/restart combinations remain unperformed; name discovery never grants connection authority. |
| M4 — reuse Base, cache and OCI data | Real Packer/HCL2/external shell and Base import are implemented. Actual Packer dependency downloads await ordinary reviewed configuration. Creation-time cache enrollment, independent generation reuse, collection/history/recovery/clear and Env-local emptying exist. Late enrollment and unknown-copy cancellation are follow-ups. Broader OCI acceptance and representative large-repository measurements are deferred; the streaming candidate removes the 32 MiB single-pack restriction with local real-Git evidence; installed acceptance remains pending. |
| M5 — understand retained data and resume development elsewhere | Inventory, component deletion diagnosis, reviewed retained-object cleanup, export/import, restored-tree comparison and latest restore by source name are on main. Windows CI passed public reclaim plus retained Workspace/OCI/snapshot restore. Required-current-data selection, guest-visible owner checks, restored authenticated development and the dedicated local reclaim-start observation remain distinct acceptance/follow-up items. Preserve unknown ownership and failed receipts. |

[CLI language scope](../reference/cli-language.md),
[Workspace input](../design/workspace-input.md),
[Packer builds](../design/packer-base-builds.md),
[cache generations](../design/cache-generations.md), and
[data evacuation](../guides/data-evacuation.md) define the actual supported operations.
Do not expand a pending acceptance item into an unrelated compatibility project.

## Acceptance priorities and limits

M5's named current-data selection and aggregate comparison helper is implemented
on a development branch for [#703](https://github.com/SLktEx/Hacocoon/issues/703).
It reuses the existing portable manifest comparer and preserves partial/unreviewed
outcomes. The remaining selection work is choosing the operator's actual required
data and checking independent retention, owner namespaces and ordinary resumed use.
No old-version reconstruction or automatic deletion is added.

Complete ordinary Packer setup/download/build/publication and reuse first when the
required communication configuration is available. The proposed three-source
require-approval configuration awaits user input after automatic approval review
refused its all-Environment scope; it has not been applied. Do not bypass that refusal
with a test allow rule. Continue independent usability work while it is pending.

The development `--builder <env>` option now allows naming one build target for
ordinary administrator rules, without adopting an existing Env or changing Policy.
This removes the need to know a random name in advance. The rejected broad rule
has not been applied; the new scoped configuration and actual installed Packer
build/reuse still need their own review and evidence. It is a usability improvement
within M4, not proof that dependency downloads have succeeded.

Reclamation succeeded with measured allocation recovery in Windows CI, including
retained Workspace/OCI verification. The dedicated local start previously failed
at enrollment while a direct Windows observation passed; a later Japanese read-only
status succeeded after normal installation. That does not explain the earlier
failure or prove a new start. An attempted global WSL restart was stopped by the
pre-action guard after an unrelated running distribution was observed; no global
shutdown occurred. Later, restarting only the dedicated WSL preserved the failure.
Read-only comparisons isolate enrollment visibility to the init interop route;
the underlying Windows cause remains unproven. Human notification/VS Code answers remain unperformed.

Current data was archived and restored to another WSL with content, raw Host owner,
link and regular-file/directory-attribute comparison. This is a small real-data
result, not guest-idmap acceptance, all-data restoration or a giant-repository claim.
Authentication, external networks and visible desktop acceptance must be recorded
as performed, failed or unperformed independently of repository tests.

Large-repository performance, additional strict validation, broader OCI/runtime
compatibility are deferred until ordinary use works. Functional Git transfer
limits remain M4 work: the incremental candidate avoids resending known history
for fetch and existing-target push. The later new-branch candidate also reuses one
advertised ancestor through an independent exact-ref read. The streaming development candidate replaces the 32 MiB new-pack
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

The network-language follow-up (`0387258d`) adds M1/M3 result and next-action
guidance while keeping JSON, source/target authority and rule scope unchanged.
Local full checks passed, including real local TCP/UDP listener cancellation;
installed/upstream acceptance is separate. The cache contract also removes stale
claims that retained-source maintenance and added-data transfer are unimplemented.
Existing native cache collection/reuse/emptying evidence is reused; late enrollment
and unknown-copy cancellation remain follow-ups rather than being confused with
the already working creation-time workflow.
