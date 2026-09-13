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
| Temporary execution — after basic editor development | Product captured run and cleanup exist. Streamed stdin/TTY is implemented on the development candidate, with scoped Incus 7.0.1 pipe/PTY and ordinary Windows ConPTY acceptance at `9767fd93`. Populated OCI acceptance remains separate; explicit persistent Workspaces survive. |
| D — understandable permissions and optional AWS | Configuration editing, saved choices and review exist. Complete fresh OS notification decisions, Linux activation, broader saved-choice/provider acceptance and authenticated real AWS listing/download. Resolve ambiguous external outcomes without blind replay. |
| E — retained data, snapshots, copy, Base and explicit cleanup | Keep new-Env restore/copy and reviewed deletion. Broader application consistency, build concurrency/history/import, partial collection/import recovery and full installed image-cleanup acceptance remain. |
| F — allocation recovery and operability | Public Windows/WSL reclaim has measured CI acceptance. Existing-installation, actual interrupted-worker review, power-loss/session/Job breadth remain. Diagnostics, reinstall/upgrade and an optional management UI are separate scope. |
| G1 — managed Env transfer | Public stopped export/import and projected Windows file route exist; stopped containerd image/writable-data transfer has native acceptance. Authenticated imported Git, broader OCI/application consistency and native Windows/direct DrvFS publication remain incomplete. |
| G2 — inventory and readable-data evacuation | Read-only native/catalog/file inventory and explicit tree archives exist. [Native image retention](../guides/data-evacuation.md#retain-ordinary-incus-images) is partial G2/G3, with two split-image transfers accepted. Classify all data, resolve incomplete/unprojected references, coordinate writers and capture required settings/manual/external data. Snapshot failure must not prevent readable-file evacuation. |
| G3 — reconstruct and compare elsewhere | One managed cross-WSL fixture and two native split-image imports passed. Unified-image import and new-Env boot from those images remain unverified. Whole-installation restore, guest-visible ownership/link/ACL/xattr comparisons, credentials/settings and broader application/client acceptance remain. |
| G4 — reviewed replacement | Planned. Retain the old WSL/data until required independent restore and comparison succeed. Inventory or one imported bundle never authorizes deletion. |

`switch-base` is disabled and its return is not planned. Ordinary delete/create
preserves retained Workspace and OCI data while selecting another Base.
Historical [ADR 0011](../adr/0011-managed-workspace-base-switch.md)
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
| M0: combine existing improvements | **partial**. Candidate `codex/roadmap-m0-m1` starts at dev/2.x `ac67fadb` (#576/#579), merges dev/v2 `4d74dd53` (#569/#571) as `5e43c8db`, main `9527948b` (#572/#574/#578) as `e941c080`, and #580 `dd7141c1` as `63822b94`. Existing dirty main checkout is retained. Integration target is dev/2.x; no main merge or distribution. Combined Incus 7.0.1, Ubuntu and Windows acceptance at `0c79f820` passed; see exact evidence. |
| M1: ordinary Windows development | **partial**. BAT result/wait (#573), hierarchical bilingual help (#575/#577), daily failure guidance and native failure grouping (#582, roadmap R1) are implemented in the candidate. Windows UI-to-Host language transport and verified Incus 7.0.1 packaged/native gates pass at `0c79f820` (English Windows). Individual argument/option help is implemented in #592. Remaining: full result translations, Japanese Windows flow, original repeated-SSH-failure route and installed long-input/resize verification. |
| M2: multi-repo Git and GUI decisions | **partial**. Workspace prepare/reopen/fork and scoped native acceptance come from #579. All-branch fetch (#584) and separately approved new-branch/fast-forward push (#586) are implemented on development branches. VS Code GUI responses (#568) are implemented on `codex/gui-approval`, using common saved scopes and private selection sessions. Durable push status and read-only reconciliation (#470) are implemented on `codex/git-push-reconciliation`; unknown results are preserved without replay. Native Git/GUI acceptance and Windows notification-contained answers remain. Main pushes still require the user's decision. |
| M3: corporate network | **partial**. Reuse #576 TCP/UDP plus its expiry/revocation/generation tests. Temporary-run stdin/TTY is implemented on the development candidate, with Incus 7.0.1 pipe/PTY and ordinary Windows ConPTY gates passing at `9767fd93`. The dedicated Windows-service timeout, VPN/DNS modes and client stream forwarding remain. No firewall exception is introduced to obtain a pass. |
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

Review: [PR #583](https://github.com/SLktEx/Hacocoon/pull/583), candidate head "1d2957eb" (development only). Focused CLI/notification/catalog/control API/composition race tests passed. Native BAT component also verified waiting with open stdin, key release and preserved failure exit 37; Explorer launch remains unverified. Existing Windows/Ubuntu installer and Incus PR gates now include dev/2.x, retaining their read-only permissions and unchanged acceptance sequence.

Further M1 work: the candidate now shares signed Incus 7.0 LTS package selection,
patch-series preferences and server verification between Ubuntu/WSL installers
and Incus CI (#479, ADR 0063). Host doctor fails clearly on unsupported versions.
Repository helper/host-preparation/package/diagnostic tests passed; fresh native
7.0 installation was pending at that point. Six-series acceptance is historical;
the current packaged baseline and export CLI contract require 7.0 LTS.

`195172f4` passed full test CI 34714239387 and packaged Ubuntu acceptance
34714239415 after correcting stale horizontal-help assertions. Earlier failed
runs remain in acceptance evidence. Windows key-wait validation was corrected
from pipes to a ConPTY component; local ConPTY passed, while Explorer acceptance
and the updated Windows CI are still required. These do not complete M1.

At cc18a60b, full test CI 34715459033 passed; real Incus standalone and Core
lifecycle/egress passed. Fresh Ubuntu run 34715458982 installed/verified Incus
7.0.1 but failed boot-guard adoption: the guard recognized only Ubuntu's daemon
path. The follow-up recognizes the exact vendor binary while retaining root,
namespace and MainPID checks; unknown active daemons fail closed. Its 20
regressions passed; installed acceptance must be rerun.

The remaining M1 client work also shares the help renderer with haco-host and
removes development-branch OS locale initialization. Windows installer component
checks passed after removal. Language transport and full option/result coverage
remain pending. Local ci-local.sh e2e printed PASS for all four stages, then
reported a shell parse error after its source had been edited during the long
run. The invocation is recorded as failed; separate current-file syntax checks
and full CI success do not erase that local harness result.

Incus run 34715459013 then failed the detached Store deletion-confirmation
test because it supplied a pipe to the terminal-only confirmation path. The
fixture now uses a private PTY for reviewed decline/approval. Lower-cost PTY and
product confirmation regressions cover that boundary; installed Store maintenance
must pass again before this gate is accepted. See acceptance evidence for the
original failed run and the successful narrower jobs.

### M2 all-heads read candidate

Roadmap R2 is now [#584](https://github.com/SLktEx/Hacocoon/issues/584).
The independent `codex/git-all-branches` worktree starts from M1 `3486b760`.
Implementation `d93f61fb` and parent integration `51220424` are pushed in
[PR #585](https://github.com/SLktEx/Hacocoon/pull/585), stacked on #583. Current
Actions branch filters exclude this temporary base; installed acceptance must run
after retargeting to `dev/2.x`. This is development-branch implementation only.
The Standard helper/agent/broker implement all-head discovery, exact-ref read
checks and ordinary branch switching. A narrow read deny still wins; main
push retains separate fixed-commit approval. No user configuration is migrated
automatically. Paired Git guides explain explicit read scope and older Workspace
fetch mappings. New-branch push, GUI decisions and native/large-repository
acceptance remain open. Per-head transfers can repeat history; M4 must measure
and improve this within the 32 MiB aggregate batch limit.

Real Git component regressions pass alternate/multiple/newly advertised heads,
branch switching, narrow allow/deny scope, no unauthorized Host object fetch,
stale/deleted/unknown ref refusal, cancellation, default-main deny/approval and
fixed-commit replay protection. Existing Git module tests and vet passed. These
run on local Git in the dedicated WSL, not an installed Env or authenticated
external Git service. The maintained `bash tools/ci-local.sh test` passed against
an unchanged source snapshot in the dedicated WSL's Linux temporary filesystem:
all Go tests/vet, Python prerequisites and 27 client tests. The earlier DrvFS run
failed at the ten-minute milestone-package timeout while copying the repository;
that failure remains recorded separately. Next: review the R2 slice, add new-branch
push and GUI decisions; preserve M1 native failures independently.
The focused Git broker and capability race tests also passed.

### M2 development-branch push candidate

Roadmap R3 is [#586](https://github.com/SLktEx/Hacocoon/issues/586).
`codex/git-branch-push` starts from R2 `76bb498d` and implements one new branch
or one existing fast-forward target per push. It reuses Standard Git preparation,
the common approval service and exact ownership checks. New creation binds a
zero old OID and an expected-absent remote lease; saved creation and update
decisions remain separate and cannot grant main. ADR 0066 records the boundary.
Real Git regressions pass denial, fixed-commit creation, separately approved
updates, main approval, competing different/identical creation, replay and
force/delete/multi-ref refusal. Malformed input and porcelain receipt tests pass.
This is local component evidence, not native GitHub or installed-Env acceptance.
The maintained `bash tools/ci-local.sh test` passed on an unchanged Linux
temporary-filesystem snapshot: all Go tests/vet, Python prerequisites and 27
client tests. Git broker/common capability race tests and documentation consistency
also passed. The [development checkpoint](versioning-and-release-status.md)
records this implementation slice; it is not a release or M2 completion.
Next: a separate stacked PR, installed Git acceptance and GUI-contained decisions.
Pack/huge-repo work remains M4.

### M1 native follow-up

Latest M1 evidence: candidate `96bbbdf8` passed full test CI 34717575075 and
all enabled real Incus jobs in 34717575098, including the corrected Store
confirmation. Private registry remains skipped. Windows run 34717575063 passed
installation/restart, pinned SSH, stopped resume and real VS Code read/write/
terminal use; pending-review, preview and export probes failed. Linux trim passed;
Windows reclaim lacked the failed transfer's manifest and notification was skipped.
Ubuntu installed successfully but used a stale privileged doctor command as an
ordinary user; `377ceb6f` changes that check to the controller-backed product CLI.
The approval fixture now supplies a real PTY and export failures gain only fixed,
redacted categories. Local terminal regressions (six) and Windows diagnostic
allowlist/child-timeout/nonzero-exit tests pass. The Windows timeout fixture emits
its marker before a slow PowerShell startup; its earlier local failures remain
test-harness failures. Next: rerun the corrected native gates, investigate export
from the categorized evidence and complete language transport. M0/M1 stay partial;
no main merge or distribution is claimed.

The next M1 slice forwards only normalized `en`/`ja` from the WSL client into
one trusted Host shell (`HACO_UI_LANGUAGE`), with request validation before Host
preparation. It leaves OS locales, JSON/exit codes and Environment execution
settings unchanged. ADR 0065 and paired language/transport/Host documentation own
the contract. Automatic Windows language selection and full translation remain
open. At `5fe184a6`, Ubuntu packaged acceptance and full test CI pass; Windows
pending-review and preview also pass after the PTY correction. Volume export
still fails and blocks its dependent Windows reclaim fixture. Next: finish the
language handoff's native acceptance and repair the Incus 7 existing-target guard for the private
anonymous export descriptor, then rerun the affected native path.

Language selector/fuzz seeds, product JSON/exit behavior, Host help, management
API rejection and real Linux shell non-forwarding regressions pass. Initial
tests using a getter that returned a POSIX locale for every environment key were
corrected to distinguish the normalized override. Incus/runtime and architecture
regressions also pass. `5fe184a6` real Incus run 34719977784 subsequently passed all
enabled jobs; private registry stayed skipped. This does not accept the unpushed
language change or resolve Windows volume export.

`3b8eefce` committed normalized language transport and `3cac2e95` committed the
private-descriptor Incus 7 export adjustment on PR #583. Test and packaged Ubuntu
CI pass at the latter head. Its Btrfs export failed because the separate Core CI
setup still installed 6.0.5; prior Core/Btrfs successes are 6.x evidence, not 7.x.
That setup now uses the canonical signed LTS helper and bounded version check.
Next: rerun Core/Btrfs on 7.x, finish Windows transfer/reclaim/notification
acceptance, automatic Windows language selection and GUI-contained decisions.
See [acceptance evidence](acceptance-evidence.md) for the failed cleanup and
version correction. Windows run 34721760573 subsequently passed transfer,
reclaim (7.93 to 4.03 GB allocated VHDX), retained-data restore and notification
routing on Incus 7.0.1. Human toast/GUI decisions and VPN/NRPT remain skipped.
M0/M1 remain partial; no main merge or release occurred.

Normal interactive Windows/WSL entry now selects the Windows user's presentation
language unless explicitly overridden. The fixed system query is bounded and
falls back to POSIX selection; the selected value reaches both the entry notice
and the existing Host-session API. No OS locale or Env configuration is changed.
Local Windows PowerShell returned `en`; the dedicated existing WSL's direct-EXE
probe failed with `exec format error` and its WSLInterop registration was absent.
That existing installation was left unchanged. Component selection/fallback and
the packaged user-path assertion are separate from native acceptance. The next
packaged Windows run must compare the Host value with the Windows setting.

The GUI slice PR #588 is stacked on PR #587 (new-branch push,
implementation `7bdd3db6`, merged parent `9d9e67ec`) and PR #585 (all-branch fetch,
merged parent `bc78d535`). Keep native GUI results distinct from the completed
`0c79f820` M1 acceptance. Existing read-only Git fetch and separately scoped main
push authority remain unchanged. Stacked work-branch PRs now use the same maintained
CI, so their changed packages receive native checks without merging to main.

At `e7ba7987`, all four maintained workflows passed, including real Windows
VS Code panel rendering and installed-controller stale-request refusal. Fresh
human GUI answers, notification-contained Windows answers and native new-branch
Git acceptance remain open. See [acceptance evidence](acceptance-evidence.md).

Issue #589 tracks temporary-run stdin/TTY. Its preparation found retained-run
cleanup selected only an Environment name. The development fix binds creation
and cleanup to one durable identity through the canonical lifecycle, blocks
unfinished-run name reuse, and preserves ambiguous legacy records for recovery.
[ADR 0068](../adr/0068-ephemeral-run-creation-ownership.md) owns the decision.
PR #590 (`9f4cf510`) passed focused/race tests and all four maintained native
workflows, including captured-run exit, retained data and cancellation cleanup.
The next development candidate implements stdin/TTY through that same lifecycle,
with bounded input credit, separate outputs and confirmed early-exit drainage.
The early-exit Unix reset regression now passes repeated race checks. Real streamed
Incus/Windows acceptance passed at `9767fd93`; M3 remains partial. Continue client forwarding and corporate
VPN/DNS gaps independently. [ADR 0069](../adr/0069-bounded-process-streams.md) owns
the transport decision; M4/M5 remain in scope.


M1 individual help is pushed as `c7169760` on `codex/cli-help-details`, PR #592 following #591.
Individual product/Host help now describes arguments, options, defaults and
requirements in both languages; product FlagSet descriptions reuse that catalog.
Setup/configuration/approval/doctor/reclaim/version help returns locally, and
remaining long subcommand usage lists route through the shared hierarchy.
Human result/error translations and Japanese Windows acceptance remain partial.
The obsolete switch-base return hint now directs users to normal recreation;
no return is planned. Focused CLI/catalog/Host tests, full local test CI and docs
checks passed. Continue result translations while preserving #591's separate
Windows input failure and correction evidence.


### Retained-data operation presentation

`codex/cli-operation-messages` builds on #592. Shared English/Japanese messages
cover retained Workspace/source/Base/OCI reviews, deletion consequences and
results, snapshot headings and Workspace fork completion. The five reviewed
deletion clients share the existing confirmation rules and stop on failed warning
or prompt display. Canonical ownership/reference/deletion/cleanup remains in the
controller. Full result/error translation and Japanese Windows are still partial.
Temporary ConPTY acceptance now passes at `9767fd93`; its earlier failure remains
in the acceptance evidence. Continue M1 result coverage and independent M2–M5
work; this does not establish fresh GUI/toast, authenticated Git, Packer/cache,
large-repository measurement or complete migration acceptance.

The parent #592 Go 1.26 job exposed a local Bash PTY fixture race, reproduced
also on Go 1.27. `codex/pty-resize-race` synchronizes each command with its fresh
readline prompt; syscall evidence shows the stand-in otherwise writes a stale
window size over the transport's resize. Native Windows/Incus passes and this
fixture correction remain distinct evidence. Product terminal behavior is
unchanged. Continue the original SSH-failure and installed long-input gaps,
then the independent Git result reconciliation in #470.
