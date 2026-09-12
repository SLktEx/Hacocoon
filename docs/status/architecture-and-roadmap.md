# Architecture and roadmap

Hacocoon is a **Secure Workspace Runtime**. It gives developer tools and agents
freedom inside isolated Environments while keeping Host/external authority behind
explicit boundaries. This page owns remaining work and direction, not a second
feature-status table. Use [implementation status](../IMPLEMENTATION_STATUS.md) and
[acceptance evidence](acceptance-evidence.md) for current scope and exact results.

## Product boundaries

Core owns Workspace identity/leases, Environment lifecycle, Execution, resource
budgets, Policy/Approval/Audit, narrow capabilities and client interaction contracts.
Provider and client specifics stay behind adapters. Standard supplies maintained
replaceable defaults; Plugins provide optional integrations.

The Physical Host retains Incus and privileged platform authority. Persistent
trusted `haco-host` is management infrastructure inside the TCB, not an untrusted
Environment. Incus owns Btrfs backing, loop, mount, instance and volume lifetimes.
Hacocoon adds data associations, ownership receipts and security guards.

IDE/AI chat, model routing, task DAGs, worktree orchestration, code review and merge
decisions remain external. Cloud provider seams remain, but concrete cloud Environment
implementation is deferred. Optional OCI tooling is not a Core dependency.

## Roadmap model

Checkpoints record chronological progress; earlier partial work does not block a
later useful slice. Numbering policy/history is maintained only in
[versioning and release status](versioning-and-release-status.md).
Stages below group user outcomes and their **remaining** work. A stage label alone
is never proof of implementation or acceptance.

## User-facing development order

| Stage / outcome | Remaining work and constraints |
|---|---|
| A — one managed repository development cycle | Preserve clone → independent Workspace → Env → SSH/Git → stop/reuse. Broader hosts and repositories need their own acceptance. |
| B — multiple repositories, native Windows access and retained OCI | Preserve collection isolation and approved push. Complete broader trusted Host image preparation → independent Store copy → actual image use acceptance. Windows drive/executable breadth and interrupted-installation cases remain. |
| C — convenient daily development | SSH setup/selection, Host recipes, DNS, project setup, restricted preview and scoped doctor now exist. Remaining acceptance includes recipe recreation/cancellation, default browser, VPN/NRPT/restart combinations and broader client workflows. |
| Temporary execution — after basic editor development | Product noninteractive run and cleanup exist. Interactive stdin/TTY and populated OCI execution acceptance are separate work; explicit persistent Workspaces must survive. |
| D — understandable permissions and optional AWS | Configuration editing, saved choices and review exist. Complete fresh OS notification decisions, Linux activation, broader saved-choice/provider acceptance and authenticated real AWS listing/download. Resolve ambiguous external outcomes without blind replay. |
| E — retained data, snapshots, copy, Base and explicit cleanup | Keep new-Env restore/copy and reviewed deletion. Broader application consistency, build concurrency/history/import, partial collection/import recovery and full installed image-cleanup acceptance remain. |
| F — allocation recovery and operability | Public Windows/WSL reclaim has measured CI acceptance. Existing-installation, actual interrupted-worker review, power-loss/session/Job breadth remain. Diagnostics, reinstall/upgrade and an optional management UI are separate scope. |
| G1 — managed Env transfer | Public stopped export/import and projected Windows file route exist; stopped containerd image/writable-data transfer has native acceptance. Authenticated imported Git, broader OCI/application consistency and native Windows/direct DrvFS publication remain incomplete. |
| G2 — inventory and readable-data evacuation | Read-only native/catalog/file inventory and explicit tree archives exist. [Native image retention](../guides/data-evacuation.md#retain-ordinary-incus-images) is partial G2/G3, with two split-image transfers accepted. Classify all data, resolve incomplete/unprojected references, coordinate writers and capture required settings/manual/external data. Snapshot failure must not prevent readable-file evacuation. |
| G3 — reconstruct and compare elsewhere | One managed cross-WSL fixture and two native split-image imports passed. Unified-image import and new-Env boot from those images remain unverified. Whole-installation restore, guest-visible ownership/link/ACL/xattr comparisons, credentials/settings and broader application/client acceptance remain. |
| G4 — reviewed replacement | Planned. Retain the old WSL/data until required independent restore and comparison succeed. Inventory or one imported bundle never authorizes deletion. |

`switch-base` is disabled and on hold, with no return scheduled. It does not block
A–C. If future evidence justifies a convenience operation, reconsider its necessity,
Workspace/OCI/snapshot semantics and UX first; ordinary delete/create already preserves
retained data. Historical [ADR 0011](../adr/0011-managed-workspace-base-switch.md)
does not reinstate a public command.

## Persistent data and OCI direction

The current [Store contract](../design/persistent-oci-store.md) uses independent,
exclusively reserved data. Environment deletion releases attachment only after runtime
absence; it does not delete the Store. Ordinary creation initializes/reuses the
Workspace-bound Store automatically; `--no-oci` is the opt-out.

Processes, sockets and `/run` remain disposable. No guest-populated Store is mounted
into trusted Host, and no Host credentials or control socket enter an Env.
Completed-copy recovery must prove exact native operation completion; unknown work
continues blocking attachment/deletion. Do not propose catalog edits as a user flow.

Docker Store compatibility, runtime-version migration, simultaneous writable sharing,
large-image/performance breadth and live migration are deferred. Registry proxy,
credential broker and local registry are optional future directions.
**Local OCI Registry is not a required roadmap gate**; reconsider it only for measured
bandwidth, rate limits, restricted-network or centralized-policy needs.
Legacy Seed/distribution evidence is historical and does not define current B4.

Snapshots retain independent rootfs and managed data. Base is provenance only.
Automatic Base filesystem retention/backup was replaced by the
[current snapshot model](../design/environment-snapshots.md).
Do not build a generic recovery/storage platform or require exact old runtime/WSL
reproduction when required data plus a new Env suffices. DB volumes are deferred.

## Trusted Host and networking direction

Preserve one controller and narrow management contracts. Installer/setup must not
gain a second privileged lifecycle implementation or expose raw Incus authority in
`haco-host`. Explicit Physical Host diagnosis remains available.

Windows drive hotplug/removal/reconnection, additional executable families and
interrupted upgrades remain unverified. Setup/entry repairs only an absent native
WSL binfmt registration through WSL's own service; it does not monitor external
changes continuously. The earlier disappearance trigger remains unconfirmed.

Hostname-aware egress uses Core policy and Standard enforcement, with an Incus
lower-layer guard. DNS lookup never grants a connection. VPN/NRPT and live
firewall/network changes need actual supported-host evidence.
[Incus support/version policy](https://github.com/SLktEx/Hacocoon/issues/479)
remains a separate decision; do not infer it from a single installed version.

## Client and Git follow-ups

Clients remain replaceable consumers of [client APIs](../reference/client-adapter.md)
and [interaction events](../reference/interaction-events.md).
Agent Host/AHP breadth, other IDEs, native Windows CLI and management UI are separate
optional work, not reasons to add dependencies to Core.

Automatic SSH proxy variables and ordinary `haco open` are implemented; the old
manual-export and “SSH setup planned” follow-ups are obsolete. Current desktop
acceptance is candidate-bound and does not prove every OS/editor combination.

[Git interruption/results](https://github.com/SLktEx/Hacocoon/issues/470) includes
ambiguous remote push completion, larger packs, additional auth methods, LFS,
submodules and multiple refs. Workspace membership editing and general interrupted
preparation/recovery also remain. Native caller-owned failures must preserve exact
identities and reservations.

## Operational confidence

Maintain real Incus substrate checks independently from lifecycle/product acceptance.
Ordinary-user and installed-package checks must not be replaced by privileged
test fixtures that silently repair product gaps. Logging follows the
[shared redaction contract](../reference/logging.md).

Use [failed-run and acceptance evidence](acceptance-evidence.md) when prioritizing
follow-ups. Do not erase unexplained failures because a later run passed, or turn
a skipped external prerequisite into an implemented support claim. Old progress
diaries and superseded numbering remain in Git history.

## Active M0–M5 execution record

The user-supplied Hacocoon_Roadmap_2026-09-13.md sets the current execution order.
The earlier A–G table retains historical acceptance mapping, not a requirement to
rerun all old tests. Main, dev/v2 and dev/2.x are distinct publication states.

| Stage | Current candidate and next gate |
|---|---|
| M0: combine existing improvements | **partial**. Candidate `codex/roadmap-m0-m1` starts at dev/2.x `ac67fadb` (#576/#579), merges dev/v2 `4d74dd53` (#569/#571) as `5e43c8db`, main `9527948b` (#572/#574/#578) as `e941c080`, and #580 `dd7141c1` as `63822b94`. Existing dirty main checkout is retained. Integration target is dev/2.x; no main merge or distribution. Combined installed acceptance remains pending. |
| M1: ordinary Windows development | **partial**. BAT result/wait (#573), hierarchical bilingual help (#575/#577), daily failure guidance and native failure grouping (#582, roadmap R1) are implemented in the candidate. Remaining: full help/options coverage, language transport, Windows/VS Code/real terminal acceptance and Incus 7.0 LTS (#479). |
| M2: multi-repo Git and GUI decisions | **partial existing foundation**. Workspace prepare/reopen/fork and scoped native acceptance come from #579. All-branch fetch, new-branch push, read/write permission separation, direct GUI responses (#568) and ambiguous push reconciliation (#470) remain. Main pushes still require the user's decision. |
| M3: corporate network | **partial existing foundation**. Reuse #576 TCP/UDP plus its expiry/revocation/generation tests. The dedicated Windows-service timeout remains unresolved; VPN/DNS modes, client stream forwarding and temporary-run TTY remain. No firewall exception is introduced to obtain a pass. |
| M4: large repositories | **planned remaining work**. Preserve existing Base builder/CoW/OCI results. Real Packer HCL2 plus external shell (#566), Host-selected normal-Env cache generation/COW reuse (#570), cleanup and actual large-repo measurement (#241) remain. Small #579 fixtures are not giant-repository acceptance. |
| M5: cleanup and migration | **partial existing foundation**. Preserve snapshot/copy/export/import/reclaim and stopped-containerd evidence. Deletion diagnosis (#523), required whole-installation inventory/restore/comparison and authenticated restored Git remain. Final old-WSL replacement requires identified verified data and explicit authority for deletion. |

The integration is not a release. R1 now maps to #582. R2–R6 remain document
candidate labels; create only nonduplicated, bounded issues when their remaining
implementation scope is established. Web management, other PCs/backends and AWS
acceptance stay outside this mainline.

Validation so far: dedicated hacocoon-second Go 1.27.1 passed the maintained
`bash tools/ci-local.sh test` (all Go packages, vet, notification clients 27/27 and
Python prerequisites); focused CLI/notification/catalog tests passed after the
new changes. Native Windows BAT component tests passed 0/1/37/3010 and missing
PowerShell/adjacent-script cases. Documentation consistency passed after resolving
moved links. A wrapper's final shell exit expansion failed after all initial Go
packages passed; a direct rerun passed. An intermediate BAT test edit failed to
parse and was corrected before the recorded passing run. These are not product
SSH failures or native installer acceptance.

Next: finish candidate-level verification and PR evidence, then continue the M1
language/ordinary Windows gaps. Keep #553's reported PF-host acceptance and #579's
Windows outbound timeout distinct from later narrower passes. No source data,
existing Env, retained Store, user branch, tag or release has been deleted.
