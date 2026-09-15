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
| G4 — reviewed replacement | Deferred outside current M0–M5: old-version compatibility and migration are unnecessary during the current test phase. Code retirement does not delete data. |

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

Explicit [TCP/UDP relays](../design/network-connections.md) now preserve those
boundaries with generation-bound authorization, expiry and revocation. Broader
Windows outbound-service access and VPN remain unverified. [Workspace path entry
and stopped data forks](../design/workspace-workflow.md) reuse canonical ownership;
large-repository performance and automatic Windows/editor entry need separate acceptance.

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

## Current main integration order

The user-authorized M0–M5 work now targets `main`. Reuse development-branch work
in bounded, reviewed PRs without replacing current main improvements. Existing
worktrees and unfinished changes remain separate; no main merge or distribution
is implied by development-branch completion.

Prioritize ordinary installation, help, SSH and approval use. The first candidate
integrates #580/#583's daily bilingual presentation and vertical help; Windows
language transport, notification grouping and installer final-screen integration
follow. M2 Git/GUI, M3 transport/DNS, M4 cache/Packer and M5 Seed retirement
remain active. Local regression checks are primary; continue independent work
while required hosted checks run. Large-repository measurement and additional
validation follow usable implementation and remain unverified until executed.
Old-version compatibility and migration are outside M0–M5 by the latest user direction.
Retained-data ownership and authority checks remain required throughout.


The next M1 integration candidate includes repeated native failure grouping and
BAT completion/failure/restart results with key wait. Local notification and native
Windows BAT/ConPTY checks pass; the restricted PowerShell fixture wrapper and
remaining installed/Explorer/SSH acceptance are explicitly recorded. The next candidate also integrates
normalized Host-session language transport and read-only Windows UI-language
selection. Its local regression/race and Windows installer components pass; the
real Windows query returned Japanese. Full packaged language/GUI acceptance and
remaining command translations stay open. M5 runtime/builder/sampling retirement
is proceeding independently on current main. Per the user's 2026-09-15 scope
correction, old-version compatibility and migration are outside M0–M5.

## Current development scope

The user confirmed on 2026-09-15 that the product is still under test and needs no
old-version compatibility or migration layer. Those items are outside M0–M5;
do not keep Seed catalog readers/converters or block current usability on old-data
migration acceptance. Current-version Workspace/OCI retention, exact ownership and
authorization remain product requirements. Performance and additional validation
follow useful ordinary flows.

The main-targeted M5 candidate reuses #655–#657's runtime/harvest/builder retirement,
preserves current main's ordinary tooling and Base/OCI contracts, and removes
remaining Seed sampling/recommendation and legacy deletion/re-enable state.
No application data is removed. Local verification and PR/main status are recorded
separately from implementation; M1 Windows integration and M2–M4 continue.

## Git branch workflow integration

All-head reads and reviewed new-branch/fast-forward pushes from #585/#587 are
implemented in the main integration candidate. Keep main push approval independent
from clone/fetch. Complete GUI answers and explicit unknown-result observation next.
Authenticated installed use, 32 MiB transport replacement and large-repository
measurements remain separate; ordinary local Git tests do not prove them.

Main now includes #660 (`7e876bc1`), #661 (`44211fd2`) and Seed retirement #662
(`119e3007`). Each final head passed all five CI workflows. The user explicitly
authorized main merges after successful CI; tags/releases remain separate.
PR #663 rebases the M2 Git slice on that main. GUI and M3 streaming continue;
current data retention, DNS, cache/Packer and remaining M5 cleanup are still open.

## Daily guidance follow-up

Reuse #592/#593's detailed bilingual help and retained-data results while keeping
current JSON, portless SSH and once-per-Host recipe behavior. Deletion confirmation
stays common in the client; controller lifecycle remains canonical. Reuse #659 to
remove duplicate preliminary Host tooling. M2 push reconciliation, M3 stream/TTY/DNS,
M4 public cache/Packer and current-version M5 cleanup remain active. Old-version
compatibility/migration are excluded; new installed acceptance remains distinct.


## GUI approval integration

The optional VS Code client now reuses #588 to show current requests and complete
explicit allow/deny and saved-scope choices inside a local GUI. Fresh human answers
and installed GUI acceptance remain separate. Windows notification-contained
answers now reuse #611 and subsequent refusal/cancellation corrections; both
clients use the existing common review and Policy services. Fresh installed
answers and visible notification layout still require acceptance. Old-version compatibility/migration are outside M0–M5.


## Temporary execution on main

The M3 candidate reuses #590/#591 creation-bound cleanup and bounded input/TTY
streams in main's split lifecycle files. It preserves shared cleanup outcomes
and requires current run identity rather than adding legacy migration. DNS modes
and client-side forwarding remain independent work. Local regression and new
native acceptance are recorded separately; large-repository performance remains deferred.

The M2 reconciliation candidate reuses42aa706f for durable push receipts and exact-ref observation under fresh read authority. GUI answers remain in #664; authenticated installed acceptance and larger Git transport remain separate. No old-version compatibility is added.

The M4 public cache candidate adds Host settings, creation-time enrollment, stopped collection and named status output. Next: complete history/clearing/recovery and added-data transfer, then installed ordinary workflow acceptance. Large-repository measurements follow usable flows and are not claimed by small fixtures.

Named cache history and reviewed source clearing are implemented in the follow-up to #669. Next: recovery of positively completed copies, orphan-source browsing, existing-Env enrollment and additional-data transfer. Unknown completion remains owned; performance stays deferred.

The cache recovery follow-up completes named positive-receipt recovery and current-generation adoption through shared ownership operations. Unknown native copy reconciliation, orphan-source access, existing-Env enrollment and added-data transfer remain; performance stays deferred.
