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

Main `2f421006` / [#687](https://github.com/SLktEx/Hacocoon/pull/687) integrates the
previous Workspace, Git/GUI, network/DNS, Packer, cache, retained-data and notification
work. Main `ee8bf7fb` / [#688](https://github.com/SLktEx/Hacocoon/pull/688) adds
reclamation failure diagnostics. Both exact PR heads passed all five workflows.
These are implementation checkpoints, not M0–M5-wide acceptance or distribution.

[#692](https://github.com/SLktEx/Hacocoon/pull/692) includes #689–#691 and adds
bounded Windows SSH failure classification. Exact head `4e7a45a7` passed four Linux
workflows. Its first Windows attempt failed at an external VS Code extension HTTP503 after
SSH/transfer checks passed. The one retry passed SSH/editor and Linux reclamation,
but public reclamation refused an attached disk; notifications were skipped. Earlier
unexplained Windows failures remain separate. Main integration still needs all
five successful workflows for the exact candidate.

[#693](https://github.com/SLktEx/Hacocoon/pull/693), implementation `e1ec0894`, adds
latest-ready restore by environment name. Local tests and an installed two-save,
source-deletion, latest-restore and exact-fixture cleanup journey passed. It is a
stacked draft during #692 CI, with main as the final target. Neither PR is a release.

## M0–M5 remaining work

| Stage and useful outcome | Remaining implementation or acceptance |
|---|---|
| M0 — use existing improvements together | Integration is on main. Preserve the same-version ordinary installation → open/edit → stop/resume → retained-data cycle when changing connections. Existing Windows CI includes editor, cold SSH, forwarding and retained-data checks; wider configurations remain separate. |
| M1 — understandable everyday use | Vertical help, OS/Host language handoff, repeated-failure grouping, installer final results and detailed daily guidance are integrated. Candidate #690 adds Japanese reclamation results. Remaining command families and original tool diagnostics are listed in the language contract. Fresh human GUI/layout answers, the observed managed-user systemd-session warning and broader Windows entry remain acceptance work. |
| M2 — independent multi-repository work and reviewed Git | Selected membership in a new fork, independent checkout/linked-worktree input, all-head fetch, reviewed new/fast-forward push and exact old/new reconciliation are integrated. Complete actual authenticated development and fresh Windows-notification/VS Code GUI answers. Keep main push approval independent of clone/fetch. In-place membership editing is not required for the implemented independent-fork flow. |
| M3 — permitted communication outside ordinary networks | TCP/UDP, client loopback forwarding, interactive temporary execution and `host/backend/disabled` DNS selection are implemented. DNS selection survives snapshot/copy/import. Complete end-to-end guest DNS/Policy checks on the supported baseline and actual VPN/NRPT/restart combinations where available; name discovery never grants connection authority. |
| M4 — reuse Base, cache and OCI data | Real Packer/HCL2/external-shell execution and Base archive import are wired. Actual Packer builds remain blocked at dependency downloads under the dedicated installation's default-deny Policy; use normal reviewed settings, not test overrides. Creation-time cache enrollment, generation reuse, collection/history/recovery/clear and Env-local emptying are implemented. Late enrollment and unknown native-copy cancellation remain separate follow-ups. Broader ordinary OCI use and real large-repository measurements remain. |
| M5 — understand retained data and resume development elsewhere | Component-level snapshot deletion diagnosis, retained-object cleanup, export/import, reclamation diagnostics and current-data inventory/archive are implemented. Candidate #690 adds restored-tree comparison and has a limited actual-data restore pass. A later development change restores by source environment name using the latest recorded ready capture, without requiring an internal ID for ordinary use. Complete required-current-data selection, appropriate guest-visible owner/attribute checks, restored editor/build/OCI/authenticated Git use, and the dedicated installation's reclaim-start diagnosis. Preserve unknown ownership and failed receipts. |

[CLI language scope](../reference/cli-language.md),
[Workspace input](../design/workspace-input.md),
[Packer builds](../design/packer-base-builds.md),
[cache generations](../design/cache-generations.md), and
[data evacuation](../guides/data-evacuation.md) define the actual supported operations.
Do not expand a pending acceptance item into an unrelated compatibility project.

## Acceptance priorities and limits

Complete ordinary Packer setup/download/build/publication and reuse first when the
required communication configuration is available. The proposed three-source
require-approval configuration awaits user input after automatic approval review
refused its all-Environment scope; it has not been applied. Do not bypass that refusal
with a test allow rule. Continue independent usability work while it is pending.

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
for fetch and existing-target push. The 32 MiB new-pack limit and complete
new-target push remain known constraints, not completed giant-repository support. Measure representative
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
