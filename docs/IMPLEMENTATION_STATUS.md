# Implementation Status

At 7517c27 all applicable Incus and normal-test jobs passed; Windows VS Code passed, but transfer failed installing Git over SSH (exit 100). SSH preparation now configures the current managed proxy for sshd sessions; local tests/vet and installed Windows Git-over-SSH preparation passed at 684e411. See [ADR 0058](adr/0058-ssh-session-egress-environment.md).

GitHub-route import reconnection uses existing source clone and Git connect commands. Component acceptance and package race/vet passed for matching routes, mismatches, offline refusal and same-name replacement; native imported Git fetch/push remains unverified. See [the owning contract](design/git-and-github-capability.md#reconnect-an-imported-github-workspace).

Windows transfer at 0cc27a5 failed during seed-repository (exit 127); VS Code passed. The fixture now installs missing Git in trusted Host and the source Env through normal package routes. Transfer subsequently passed at 684e411; the independent approval probe failed.

## Public Environment import in progress

Status: **partial**. Linux `haco env import <file.haco> [new-env]` now connects the
client file reader, management upload, native Workspace/OCI owners and canonical
Env creation/start. The default name is SOURCE-imported; existing names are refused.
The input remains unchanged. Version-1 and source Host file routes import offline;
GitHub descriptors preserve routing only, without approval or credentials.

Current composition configures private staging and one 64 GiB payload budget for
all native import adapters. Fresh resource owners, Env generation and current
sandbox/managed SSH setup are retained. No Base filesystem, automatic backup,
replacement of current data, import catalog or schema migration is added.
Failure receipts identify retained data; startup failure leaves it for inspection.

Before public CLI wiring, the internal Incus/Btrfs aggregate passed in 558.35s at
6360a23, with independent rootfs, two Git Workspaces and OCI imported after source
deletion, running Env and owned cleanup. Typed upload race tests passed in 4.808s;
all Go/vet/27 JS tests, docs and workflow policy passed at 2992c47. The all-entry
local CI attempt failed because Ubuntu lacked pwsh; later all-entry stages were
not executed. This is separate from latest-head GHA.

At b7297a3, dedicated Incus/Btrfs execution passed the shipped import CLI, management
stream and canonical importer: independent rootfs/Git/OCI, real startup, old-generation
refusal, managed SSH reset, retention after Env deletion and owned cleanup. This used
a fixture controller; installed-controller/desktop import subsequently passed at 684e411. Overall
aggregate completion, including subsequent snapshot/copy checks, is recorded separately. Live SSH passed at 684e411;
live OCI consistency, Git reconnection, incomplete collection cleanup and native
Windows file input remain unfinished. See [Environment transfer](design/environment-transfer.md#linux-import-command).

The first complete public-import aggregate at b7297a3 passed export, native import
and restore, then failed during public copy at the fixture's 12-minute deadline
(720.07s). This is an overall FAIL, not a successful aggregate or SKIP. Its ownership
catalog and saved data remain at `/var/lib/haco-snapshot-aggregate-1920048809` for
explicit cleanup; shared data was not selected for deletion. The expanded test
sequence now has a 20-minute fixture budget and a 25-minute GHA test-process budget.
Product timeouts and isolation are unchanged. At b7297a3, GHA's real Incus/Btrfs
[aggregate step](https://github.com/SLktEx/Hacocoon/actions/runs/34455660292/job/102801320149)
succeeded with the shipped import CLI and all aggregate assertions. This provides
independent acceptance while preserving the local failure record. The extended
local-budget variant has compiled but has not been rerun locally; all four b7297a3 workflows passed. The follow-up fixture-budget commit requires
its own latest-head CI result.

The bare shipped controller passed native import/data/owned cleanup in 20.35s at
a58d553, but the full gate failed on diagnostic-directory layout, now corrected.
SSH attempts at 6d5e027/e598270 failed; the latter confirmed missing sshd and SSH
provisioning failure in the bare fixture. SSH continuation is now wired into the
existing installed Windows gate with a separate managed source, normal scoped
package Policy, public export/import, fresh pinned Windows SSH and retained-data
recreation. This installed transfer gate and native controller checks passed at 684e411; both remain
required. See [the acceptance record](design/environment-transfer.md#installed-controller-and-ssh-acceptance).

## Public Environment export in progress

Status: **partial**. Linux `haco env export <stopped-env> [file.haco]` now uses the
management stream and verified, no-overwrite client publication. Default output
is `<env>.haco`; no separate snapshot command or controller path is required.
Unix stream and real-filesystem CLI race tests passed. Local shipped CLI full gates failed on fixture deadlines after export passed;
the equivalent GHA aggregate gate passed in 47.06s at `3d0dd9a`, with all four
applicable workflows successful; public import acceptance and native Windows output remain pending. See
[the owning contract](design/environment-transfer.md#linux-export-command).

The internal stopped-Env exporter now composes canonical capture/read/delete,
native component producers and anonymous whole-bundle staging. Linux public CLI
and controller artifact delivery are now partial. Dedicated native aggregate export
acceptance passed in 314.12s; this does not prove public bundle import or SSH. See [Environment transfer](design/environment-transfer.md#internal-stopped-environment-export).

## Detached Store maintenance in progress

Status: **partial E5**. Existing image list/delete commands accept retained Store
IDs for nerdctl on Linux/WSL amd64. Production composition provisions pinned OCI
tools automatically before attaching retained data. Canonical maintenance runs
preserve Store reservation, the original Workspace association, fresh Env identity
and uncertain-cleanup evidence. No new user command, schema, backup or recovery
state is added; detached Docker and candidate-selected GC remain unimplemented.

At `bd1c9a5`, all four GHA workflows passed. The real Incus/Btrfs shipped-controller
and CLI gate [passed in 588.51s](https://github.com/SLktEx/Hacocoon/actions/runs/34417051340/job/102684134054):
detached inventory, referenced-image refusal, confirmed digest deletion/absence,
retained container metadata, canonical temporary cleanup and explicit owned Store
cleanup. This uses a bare root controller/private socket with a real isolated
catalog. Full installed Standard-egress, ordinary-user/desktop acceptance and
other tooling architectures remain unverified or unsupported.

Earlier controller gates failed. Corrections use the canonical provider ID,
remove contradictory SkipDefaultResource from explicit Store creation, and give
the root fixture a private TMPDIR without weakening lock ownership validation.
The real-catalog/lifecycle regression reproduced the explicit-Store failure before
the fix and verifies reservation and cleanup on success and operation failure.
Those failures are recorded in PR #514, not reclassified as SKIP.

Earlier dedicated native tool delivery and image operations passed in 237.37s;
empty-cache HTTPS acquisition passed in 70.71s without running downloaded binaries
on the Host. Related race/vet checks passed. Private-registry acceptance is
workflow_dispatch-gated SKIP. See [the owning contract](design/oci-image-deletion.md#detached-store-implementation-in-progress).

## Environment transfer prerequisites

The Linux/WSL Incus adapter now exports an owned saved Workspace/OCI volume into
an unnamed read-only archive, checking native ownership and backup cleanup. Its
dedicated Incus 6.0.5/Btrfs adapter test passed in 5.92s; local race and vet passed. Linux public export is connected; public import acceptance remains pending. The internal rootfs producer now uses a uniquely owned native image and an anonymous archive; dedicated Incus 6.0.5/Btrfs adapter acceptance passed in 13.44s.
See the [owning contract](design/environment-transfer.md#native-saved-volume-export-adapter).

Public G1 remains **partial**: Linux export/import are implemented, with public import acceptance pending. Internal snapshot/archive inventory
matching covers all currently supported Workspace components and optional OCI;
the saved-source read boundary now shares canonical deletion locks and verifies
retained components. Native archive production and Linux export are connected; public import acceptance remains pending. Opt-in native Incus rootfs/volume
archive tests and their existing-GHA integration are implemented. The dedicated
Incus 6.0.5/Btrfs run passed in 11.24s after correcting fixture path/namespace
assumptions; source/destination independence, Git state, links, mode and retained
archive checks passed. Public rootfs import and authority reconstruction remain
unimplemented. A separate empty-rootfs image round trip passed in 14.88s with no
Base/image source, source instance/image removal before import and fresh explicit
configuration. An extended dedicated WSL Incus/Btrfs run passed in 22.44s,
checking positive source-image absence and a fresh destination file read after
image deletion, plus retained archive immutability. This stopped-instance fixture
does not prove running-container OCI deletion or OS/SSH/public-import acceptance. See
[the owning contract](design/environment-transfer.md).

An internal fixed-role streaming envelope writer/verifier now checks complete
bytes without extraction or Incus effects; focused race tests and vet passed.
Linux/WSL staging now retains verified bytes in an unnamed read-only file; real-
filesystem race tests and vet passed. Btrfs staging and public lifecycle integration
remain unverified/planned respectively.

## Current Incus-first snapshot contract

Status: **implemented for capture and restore into a new Environment**. The
current command is `haco snapshot restore <snapshot-id> [new-env]`; in-place
replacement remains **planned**. Earlier checkpoint entries below describe the
acceptance at that revision: their former “public restore planned” statements do
not override the implemented command. See the [owning contract](design/environment-snapshots.md#restore-into-a-new-environment).

Incus owns independent rootfs/volume copies and runtime operations. Hacocoon adds
aggregate consistency, retained data ownership and fresh security generations.
New saves contain rootfs, Workspace, optional OCI and metadata, with no Base
filesystem or automatic pre-restore backup. Schema 13 retains legacy saved
Base/backup ownership and source reservations; ordinary upgrades need no manual
saved-data rewrite. Existing records are not silently discarded.

[PR #493](https://github.com/SLktEx/Hacocoon/pull/493) records real Incus/Btrfs
Base/cache-independent capture and preparation. [PR #501](https://github.com/SLktEx/Hacocoon/pull/501)
records real public restore after source deletion, fresh generation, retained
Git/OCI bytes and exact-owned cleanup. These are executed results at those
revisions, not new acceptance of every later change. Restored SSH handshake and
live OCI database consistency remain unverified. Reclamation, detached Store
image maintenance and migration are separate unfinished work.

## Host-source image operations

Partial implementation: `image list/delete --host` selects only the exact managed Host source. The current plugin/controller/CLI route and Incus adapter enforce source ownership, local role/mount/layout, unprivileged running state, the existing Host-copy operation lock and a fixed command/template allowlist. No guest Store can be used as a Host source. Five related package tests passed; focused security/CLI regressions are included. Four related race packages passed, including the shared Host-copy lock guard. Additional uppercase-tag reference regressions passed. Dedicated WSL Incus/Btrfs acceptance passed in 487.19s: both runtimes listed the Host source, refused a stopped-container reference, removed only the selected image, retained other images, then completed independent Store copy/deletion and exact fixture cleanup. This uses the real Host adapter with a fixture catalog; installed-controller/public-CLI native acceptance remains unverified. Existing independent copies remain; detached Store acceptance is recorded above and candidate GC remains planned. See [the contract](design/oci-image-deletion.md#managed-host-source).

## Attached-Store image operations

Partial: image list/delete is connected to the current OCI plugin, controller and product CLI. Runtime inventory/removal replaces legacy Seed selection semantics; reviewed Env generation and Store ownership guard execution. All seven related packages passed after correcting initial controller wiring, regexp and invalid-RPC error classification failures. Documentation checks passed. Dedicated WSL Incus/Btrfs native COW acceptance passed in 417.80s: Docker and nerdctl inventory, stopped-container refusal, immutable runtime ID removal, positive absence and source-copy image independence. The isolated project/pool and its now-empty catalog were cleaned up. Four related race packages passed; a focused concurrent Env-delete exclusion regression also passed. This fixture uses a test execution adapter, so installed-controller/public-CLI native acceptance remains unverified. PR #508 merged as `3aa8b07f` after all four workflows succeeded on `4d9038b7`. Maintained local docs, policy, Go tests/vet, JS and E2E passed on exact files in WSL ext4. Systemd verification first failed on an older Ubuntu tool and passed on dedicated Hacocoon WSL; the initial failure remains recorded. Local packaging was skipped for missing tools, with GHA release-config passing instead. Detached Store acceptance is recorded above; candidate GC and reclamation remain incomplete. No schema migration. See [the owning contract](design/oci-image-deletion.md).


## Explicit source repository cleanup

Partial E5: `haco repo list [--json]` and `haco repo delete [--yes] <id>` are implemented. Referencing Workspace records protect the current Git transport. Existing registry and Host-operation locks preserve identity, pending Host-copy state and native child saved objects. Schema 13 and independent data remain unchanged. Related package tests passed. Dedicated WSL Incus/Btrfs public source CLI acceptance passed in 30.43s: Workspace reference refusal, child snapshot and Host mount retention, stale review refusal, exact detach/delete and positive absence. The isolated project was removed; shared image/pool and the ownership receipt were retained. A subsequent guard rechecks queued Git requests under the registry lock; final race/CI results are tracked in the implementation PR. See [the contract](design/git-and-github-capability.md#explicit-source-repository-deletion). OCI cleanup PR #506 merged as `6903319` after all four workflows succeeded at `73175b4`; its new native regression passed in GHA in 0.71s.


The first two source-cleanup GHA candidates failed during fixture instance creation, before source deletion ran. Explicit source-image project selection did not resolve the second GHA failure, despite a dedicated WSL pass in 28.05s; the image-project explanation was therefore not established as the complete cause. The fixture now creates an empty stopped Incus instance: this test needs only a managed Host attachment, not an image or running guest. Later dependent Base/OCI checks in both failed jobs were skipped, not passed. The empty-instance fixture passed on dedicated WSL Incus/Btrfs in 13.82s, including the public CLI, reference/child/stale-owner refusal and exact cleanup. A preceding local invocation failed before test execution because PowerShell split its flags; corrected quoted arguments were used for the passing run. Related package tests and documentation checks passed. PR #507 merged as `19c4bdd9` after all four workflows succeeded on `c4842c2`; the native source fixture passed in GHA in 0.80s.

## Explicit OCI Store cleanup

Partial E5: reviewed OCI Store list/delete is implemented. The existing catalog owns identity and reservations; Incus owns volume deletion. Native child snapshots/backups/schedules, Host sources and unfinished creation block removal. Schema 13 and independent snapshots are unchanged. Six related package tests passed. Dedicated WSL Incus/Btrfs acceptance passed in 11.21s: independent COW data, source deletion, child snapshot refusal with ready state/data retained, stale owner refusal and exact deletion/absence. The initial native test failed on incorrect fixture `snapshot show` arguments; corrected before the passing run. Its exact owned leftovers were explicitly removed and the empty pool verified. Docker compatibility and broad OCI image operations were not tested. See [the owning contract](design/persistent-oci-store.md#explicit-retained-store-deletion).

## Explicit built Base image cleanup

Partial E5: `haco base list --all [--json]` and `haco base delete [--yes]
<name-or-fingerprint>` expose retained built-image revisions and explicit removal.
Incus owns images/aliases; the service adds catalog references and reviewed owner
identity. Native create/publication and deletion are serialized; Environment and
protected alias users block removal. Independent snapshot provenance does not
require retaining the original image. No schema change, retention object or
migration is introduced. See [the owning Base contract](design/base-images-and-custom-environments.md#explicit-built-image-cleanup).

Base cleanup PR #505 merged as `32dd1e4` after all four workflows succeeded at
`9d8ff82`. Real Incus/Btrfs Base cleanup and saved-rootfs independence passed in
111.30s. Local maintained CI passed on an exact committed tree on WSL ext4 after
the Windows-mounted repository-copy test exceeded its ten-minute limit. Windows
attempt 1 failed at generated-alias SSH; the same-commit retry passed. Those first
failures remain recorded in PR #505; their success is not inferred from a retry.
Workspace PR #504 merged as `4adfa81` with all applicable workflows green.

## Explicit managed Workspace cleanup

Partial E5: `haco workspace list [--json]` and `haco workspace delete [--yes] <id>`
list and explicitly delete retained managed Workspace data. The existing lifecycle
lock excludes Environment/lease users; the registry compares the reviewed owner,
records `deleting`, and retains exact member identities after failure. Native Incus
volume ownership, attachment and positive absence checks are reused. OCI Stores,
source repositories and independent snapshots remain. Create re-resolves Workspace
identity after locking, refusing same-name replacement. No schema change or new
cleanup catalog is introduced. See [the contract](design/workspace-abstraction-and-lease.md#explicit-retained-workspace-deletion).

All related package/race tests, maintained local CI and local E2E passed on the
initial candidate. Real Incus/Btrfs GHA run 34301447147 passed the public Workspace
CLI fixture (31.19 seconds), including attached refusal, Git retention after Env
deletion, exact member deletion and independent OCI/snapshot preservation. All four
applicable workflows passed on that candidate. The subsequent native-child guard
passed focused package tests and a dedicated WSL Incus/Btrfs test (23.23 seconds):
child snapshot/backup refusal, parent/child preservation and explicit owned cleanup.
Native snapshot schedules and malformed/unavailable observations are also refused.
Preflight checks every member before recording `deleting`, and repeats before each
native deletion. Final-candidate workflow results are recorded in PR #504; the
initial candidate's green workflows do not validate this later guard.
E5 Base/source repository and OCI image cleanup, F reclamation and G migration
remain separate work.

Base builder PR #503 was merged as `2ba5434` after all four applicable workflows
passed on candidate `15fed95`. The documented create command is `haco env create`.

## Base builder

Base builder validation: all five related packages passed ordinary tests and
race tests. Real WSL attempt 1 failed during live machine-id cleanup; attempt 2
was refused by the exec-only stdin decorator. Stopped Incus file transfer through
the normal decorated runner passed a scoped native probe. Attempt 3 published
`ba2ff1a2815fcc9953016151cb6407ba2505eed09671289b96af458ea67cb7a1`
and passed first Base creation/tool execution, but failed at the 600-second test
limit during rebuild. It is not a complete native pass.
Native GHA run 34297739368 passed the complete build/rebuild fixture in
70.69 seconds; Windows run 34297739417 passed installed build-to-SSH tool execution
and VS Code. Local maintained CI test and related race tests passed. The first
test workflow failed in its Incus-free Base-list fixture; that fixture is corrected
and subsequent candidate results are tracked in PR #503. The local timeout fixture initially failed cleanup because Incus reported both
stopped and running. After an ownership-checked native force-stop, canonical
deletion and positive absence checks passed for its two Environments and two
images. Diagnostic catalogs and the shared source image remain.



Implemented representative E4 workflow: definition-driven `haco base build <definition.json>` now composes normal
temporary Env execution and stopped native Incus image publication. Ownership is
stored in Incus image properties; verified aliases feed normal pinned creation.
Older revisions, snapshots and catalogs are unchanged. Focused/native/SSH
acceptance is tracked on the change; do not infer real-host success from this
implementation entry. See [the Base contract](design/base-images-and-custom-environments.md).


## Environment copy

Dedicated WSL Incus/Btrfs acceptance passed in 357.13s after the correction: stopped-source copy, source-name prefix collision, fresh generation, independent rootfs/Git/OCI after source deletion and owned cleanup. Fixture `haco-aggregate-1a17295b1a6f50e1` was fully cleaned. The expanded aggregate fixture has an eight-minute budget; product timeouts are unchanged.

Real Incus initially failed at source-state verification: Incus prefix filtering returned both source and source-copy. The runtime now selects the exact name from name/state CSV, with ambiguity/malformed-output regressions. Initial local CI passed before this correction; final acceptance is recorded on the PR.

Implemented: `haco env copy <stopped-env> [new-env]` composes existing Incus COW capture and canonical restore, with fresh data owners and permission generation. Running sources and existing targets are refused. Temporary aggregate saves are removed after the attempt; uncertain cleanup reports their IDs. No new schema or automatic restore backup is added. Validation results are recorded with the change. See [the copy contract](design/environment-copy.md).

## Public snapshot restore

Implemented: `haco snapshot restore <id> [new-env]` copies saved Workspaces/OCI,
creates a fresh canonical Env from independent rootfs and starts it. The default
name is `<source>-restored`; existing names are refused. Failed creation cleans
only owned unused copies under the Workspace lifecycle lock. Uncertain leases
retain data; a start failure keeps the published Env. No Base dependency, backup,
new catalog state or required prepared-binding CLI arguments are added.
Focused and related race tests passed, including cancellation, foreign OCI owners,
acquiring leases and incomplete published-copy cleanup. The initial dedicated
WSL Incus/Btrfs public-restore aggregate passed in 237.69s (fixture
`haco-aggregate-9b6e7be3f4718210`, public save
`snap-9a0e62a7d6e7ee1ad2b430e06658ceaa`, Workspace
`restore-7a43b3413a89ad32`). It restored through the real CLI after source deletion,
checked fresh identity and guest data, and cleaned all owned test resources.
Final-build and full CI results are recorded with the change. Shared image
deletion was SKIP because this fixture did not own a dedicated image.
Replacement switching, restored SSH handshake and live OCI consistency remain
unverified. See [the contract](design/environment-snapshots.md).

## Workspace snapshot copy source protection

Implemented: Workspace copy now reserves its saved source in the shared catalog.
Partial cleanup retains both ownership and source protection; a published copy
with failed release can retry only that release. Schema 13 preserves schema 12
runtime source holds, schema 11 OCI receipts and older saved data. No CLI command,
Base component, backup or runtime recovery state was added. State, registry, lifecycle and composition race tests passed, as did docs and
cleanup-helper checks. Dedicated WSL Incus/Btrfs aggregate passed in 184.64s: fixture
`haco-aggregate-e742fe53ad8dc2db`, save `snap-6331fdeac4c8f8350a8604af677cdbbd`,
public save `snap-4dd65c2c8d0da67de8dc358c24689332`. Source-independent normal
Workspace/OCI copies, same-name fresh runtime generation, public snapshot CLI,
data retention and complete owned cleanup passed. Shared image deletion was
SKIP (not dedicated); public restore was not covered by that run; restored SSH handshake and live OCI
consistency remain unverified. Full CI results are recorded with the change.

## Public snapshot capture and management

Implemented: `haco snapshot create <env>`, `list [env]` and `delete <id>` use the
existing controller/lifecycle/catalog and Incus copy paths. Running sources stop
before capture and restart only after a ready save; stopped sources stay stopped.
Failed or partially saved results keep their IDs and return failure. Listing
survives source Env deletion and exposes no private bindings. No schema change,
new Base storage or automatic pre-restore backup is introduced. Public aggregate
restore was not yet implemented at that checkpoint. Validation results are recorded on this change's PR.
See [snapshot usage and contract](design/environment-snapshots.md).

PR #498 merged as `d54618b` after all four applicable workflows passed at
`5cf9bb7`. Real Incus aggregate passed in 10.62s; Windows actual SSH and VS Code
editor/terminal passed. Private registry, shared image deletion, VPN/NRPT and
fresh human notification decision remain SKIP under their documented gates.


Initial real-WSL public captures failed in 116.15s and 138.25s, retaining partial
IDs. A scoped native probe identified Incus rejecting an empty boolean: clearing
`volatile.last_state.ready` to an empty string is invalid. Capture, saved-runtime
copy and restore staging now reset that state to `false`, with native-request
regressions. A separate start probe exposed the fixture's missing production
network-ownership wrapper; the E2E now uses that wrapper. Exact-owned cleanup of
first fixture `haco-aggregate-52a680aada6ca3e6` passed; only its metadata at
`/var/lib/haco-snapshot-aggregate-1076841042` remains as evidence.

The fixed native probe passed capture and restart of the previously failing
second fixture, saving `snap-bc49bb612ad51ab361840aa59488913f`. Exact-owned cleanup
of its saves, runtime/network, Workspace and OCI passed; only metadata at
`/var/lib/haco-snapshot-aggregate-4240550795` remains. Focused application/state/API/
CLI race tests and native capture/staging/runtime-copy regressions passed.

The corrected fresh real WSL Incus/Btrfs aggregate passed in 211.58s: fixture
`haco-aggregate-53f70c5d1b1341e5`, initial save `snap-2e55b4514894de3adb8f71e2ddac39fa`,
public CLI save `snap-5f11941d2fdab09a0049716565d86d29`. The actual built `haco`
ran create/list/delete through a private controller socket: running source
stop/save/resume, saved-copy verification and listing after source Env deletion,
explicit save deletion and current Workspace/OCI preservation all passed.
Complete fixture cleanup passed. Shared cached-image deletion was SKIP (no
dedicated-image permission); public aggregate restore, restored SSH handshake
and live OCI database consistency remain unverified. Full local CI/exact-head
GHA results belong to the implementation PR.

## Saved-rootfs canonical creation

Implemented internally: saved-rootfs creation now uses the normal lifecycle,
source reservation, routed receipt and bounded owned cleanup. Schema 12 preserves
schema 11 saved data and OCI receipts. Full environment/state/workspace race
tests and added failure/concurrency/migration regressions passed; docs and CI
cleanup-helper checks passed. Dedicated WSL Incus/Btrfs aggregate passed in
129.46s: fixture `haco-aggregate-b9c627e16c2da2be`, snapshot
`snap-0b4153b66307d5bce49b46befe0ee764`. Canonical routed creation reused the deleted
source name with a fresh generation, published after its receipt, released the
saved-source reservation, and normal deletion retained Workspace/OCI. Complete
fixture cleanup passed. Shared-image deletion was SKIP; restored SSH handshake,
public aggregate restore and live OCI database consistency remain unverified.
Maintained full local CI and exact-head GHA are tracked on the implementation PR.
Aggregate data-copy orchestration and public restore remain planned.

PR #497 passed all applicable GHA and merged at `62e9947`; its actual Incus
aggregate passed in 10.30s. Windows ordinary SSH/VS Code passed. Ubuntu installer
was path-filtered out; the documented private-registry/image/VPN/human UI gates
remain SKIP.

## Saved rootfs runtime primitive

Implemented internally: native independent rootfs copy without Base/image/profile
resolution, immediate ownership receipt, shared current sandbox configuration and
fresh generation/managed SSH identity. The aggregate activation coordinator and
public CLI remain planned. Focused and full Incus-package race/vet passed.
Maintained full local CI passed before the final device-mask fix (Go tests/vet,
docs/helpers, JavaScript 27/27). Real Incus first failed in 93.76s because copied
`none` devices collided with current attachment names. A low-level regression now
covers that collision and mask-removal failures; the fix removes masks only on
the newly owned runtime after its receipt. Final related tests and docs passed.

Dedicated WSL Incus/Btrfs aggregate then passed in 135.24s: fixture
`haco-aggregate-8ce0bd921c0adafb`, snapshot `snap-d401c8f07e734b02f8c14fd03293a8c1`.
Source Env/volumes and Base were absent. Native activation, fresh generation and
source guard, guest root/Workspace/OCI bytes, managed SSH authorization reset,
canonical Env deletion retaining data and complete fixture cleanup passed.
The failed fixture's native resources were cleaned through exact-owned canonical
APIs; its metadata directory `/var/lib/haco-snapshot-aggregate-4256480528` remains.
Shared-image deletion was SKIP (dedicated-image gate); restored SSH handshake,
public restore orchestration and live OCI database consistency were not tested.
Exact-head GHA is tracked on the implementation PR.

PR #496 OCI registration passed all four applicable GHA workflows and merged
at `4b06b5f`; actual Incus/Btrfs data copies and Windows SSH/VS Code passed.
Private-registry, shared-image deletion, VPN/NRPT and human notification decisions
were SKIP under the respective fixture gates.

## Saved OCI registration

Implemented internally: saved OCI volumes copy into ordinary Stores in their
saved Btrfs pool with a fresh owner and optional Workspace association. The
catalog reserves saved data, records copy completion before verification, and
releases the source on publication or positive owned cleanup. Schema 11 preserves
schema 10 and earlier supported data; old controllers reject the new format.
Focused ownership/reservation/schema race tests passed. Dedicated WSL Incus/Btrfs
aggregate passed in 108.40s: fixture `haco-aggregate-87d745d6b76c16c0`, snapshot
`snap-cfc7eae81e93e176a380b5bd171c3e77`. Original Env/volumes were removed before
Workspace/OCI registration; new owner/Workspace association, source reservation
release, durable reload, saved bytes, independent edits and canonical cleanup
passed. Shared-image deletion was SKIP. Full local CI passed before the final
ordinary-delete guard. A new regression then reproduced that missing guard as a
failure and was fixed. Final full-package race/vet for state, persistentresource
and Incus, docs and cleanup-helper tests passed; exact-head GHA is tracked on the
implementation PR. PR #495 Workspace registration
passed all applicable GHA and merged at 239b3e6. Aggregate activation/public CLI
remain planned; no live OCI database consistency claim is made.


## Saved Workspace registration

Implemented internally: same-pool Incus/Btrfs copies register as normal managed
Workspaces with fresh ownership and trusted remote/branch provenance. Completion
receipts precede verification; incomplete copies use bounded exact-owned cleanup.
Source-less legacy manifests without Git provenance remain preserved but are
unsupported for automatic registration. Runnable activation and the public snapshot/restore CLI remain planned;
OCI registration is implemented in the slice above.

Focused race tests passed. Dedicated WSL Incus/Btrfs aggregate passed in 4.09s
with fixture `haco-aggregate-42d34d51baa33c72`, snapshot
`snap-a0b22312747180b9891e790f8427c78c`: source Env/volumes deleted before
Workspace registration, durable registry reload, saved Git commits/uncommitted/
untracked files and independent edits, exact cleanup. The first attempt failed
because planning unnecessarily consulted the default profile; planning now uses
the saved pool and has a regression. Its exact-owned snapshot was cleaned through
the canonical API; original Base/Workspace/OCI absence was verified. Ownership
files remain at `/var/lib/haco-snapshot-aggregate-666738198`. Full local CI passed
(Go tests/vet, docs/helpers, JavaScript 27/27); GHA is tracked on the PR.
Shared image deletion was SKIP; runnable restore/live OCI consistency were not
executed. PR #494
passed local CI and all applicable GHA before merge at a66035d.


## Runtime ownership before configuration

Implemented: production Incus creation records its exact routed runtime immediately
after init, before device/network/resource configuration or start. The lease stays
acquiring until complete publication. The shared post-init configuration path is
ready for saved-rootfs activation; activation/data ownership transfer remains
planned. Receipt omissions, duplicates, changed references and cleanup failure
have focused race coverage. No schema, CLI or saved-data format changes.

Full local CI passed; real Incus acceptance is tracked on the implementation PR.
PR #493's final local CI and all applicable GHA passed before merge at 632484a.

## Incus-first snapshot simplification

Implemented internally: new captures save independent rootfs, managed Workspace
members and attached OCI, with Base provenance only. Ordinary create/run no longer
retains an extra Base instance. Restore preparation copies saved data without an
automatic backup or changing current data. Failure attempts bounded owned cleanup, continues independent components and
retains recovery records only when absence cannot be confirmed. Exact receipts, source write exclusion,
positive cleanup and generation/security boundaries remain.

Schema 10 migrates schema-8 restore target identity while preserving every old
backup/Base ownership record. No stored resource is deleted by upgrade. The
unpublished schema-9 prototype is rejected explicitly. Package boundaries remain
Incus adapter / Workspace orchestration / state ownership / routing; production
Base retention callbacks were removed. See [ADR 0040](adr/0040-incus-first-snapshots.md).

Focused state/service/router/provider/ordinary-create and race tests passed.
Full local CI (`bash tools/ci-local.sh test`) passed, including Go tests/vet,
documentation/helper checks and all 27 JavaScript tests. All applicable GHA passed
on PR #493.
Dedicated WSL Incus 6.0.5/Btrfs aggregate acceptance passed in 5.95 seconds:
fixture `haco-aggregate-822d7154c3c2f6dc`, snapshot
`snap-d849c57477844eef6f02e54e9c9b4364`. The test deleted its original Base and
isolated Ubuntu image before capture, saved four components, prepared independent
copies with no backup, verified unchanged current work and copied Git/rootfs/OCI
bytes, deleted the original Environment/data and checked saved independence.
Owned snapshot/staging/source cleanup passed. The first attempt refused deletion
of an image referenced by another fixture; that failed attempt's exact owned
resources were subsequently deleted and positively observed absent. Its ownership
files remain at `/var/lib/haco-snapshot-aggregate-3029918541` for evidence.

Public save/restore and runnable restore activation remain planned; this change
does not claim those features. Live OCI database consistency is unverified.
PR #492 staging was merged after local CI and applicable GHA passed; its previous
five-component/automatic-backup behavior is historical, superseded here.

## External Workspace recreation acceptance

E1 baseline passed in dedicated WSL at product 093ed159b80e: ordinary API
stop/start retained Workspace and permanent guest files; delete/recreate retained
the guest-written external Workspace file and removed guest-only state. Corrected
fixture m1-egress-708dfbc120260908 and the initial failed /tmp-marker fixture were
fully cleaned. Phase/identity regressions and local CI passed. Windows installed
E2E includes the flow. At b73f965 its fixture failed because it did not opt out
of default OCI attachment; the fixture now explicitly opts out on create/recreate.
Corrected Windows GHA remains pending. Managed Git/OCI combinations
and E2-E5 remain separate work. See [Workspace lifecycle](design/workspace-abstraction-and-lease.md#resume-and-recreate-an-external-workspace).


## Real guest AWS refusal acceptance

Dedicated WSL passed ordinary-user creation, installed guest haco selection,
source-bound unconfigured-profile refusal, failed-download preservation and
canonical cleanup at product 093ed159b80e. Controller stayed active; the fixture
Environment/Workspace are absent. Existing Windows installer E2E now includes
the check; new-head GHA is pending. Local CI and verifier tests passed. Real
authenticated AWS/positive download remains SKIP, not success. See
[AWS acceptance](design/aws-operations.md#installed-guest-acceptance).


## Ordinary guest AWS CLI

Implemented: Standard creation/start supplies the ordinary guest haco entry point.
AWS list/cp automatically use the guarded source-bound endpoint, with no --env or
credentials. Downloads reuse verified private publication. Focused race, real HTTP
queue/Policy/audit integration and setup reuse/conflict checks passed. Installed
guest acceptance remains pending; real AWS remains SKIP for missing prerequisites.
See [AWS operations](design/aws-operations.md#use-aws-inside-an-environment).


## Guest AWS server boundary

Implemented server slice: the guarded Standard listener admits only AWS list/get
requests tied to the exact persisted source Environment creation ID. Management
and approval decisions are absent. Source, recreation, spoofing and frame-size
regressions passed. Guest CLI is implemented above; installed guest/AWS acceptance remains pending.
See [AWS operations](design/aws-operations.md#guest-request-boundary).

## AWS account labels

Implemented: optional Host AWS profile labels are tied to the actual STS account ID
and included in review/saved scope. ID mismatch or changed labels refuse execution.
Maintained local CI, focused race and fifteen SDK/config regressions passed.
The ordinary controller review, saved permission and revocation tests also passed
with labels for both listing and downloads. Real authenticated AWS and
desktop rendering remain SKIP for missing Host prerequisites. See
[AWS operations](design/aws-operations.md#account-names-in-review).


## Streamed AWS object downloads

Implemented repository slice: haco aws s3 cp streams an approved current object
through the trusted Host and controller, verifies byte count/SHA-256 and the final
execution/audit receipt, then atomically publishes the local file. Existing files
remain unchanged on pre-publication failure. Scope, SDK response and filesystem
regressions passed, including 20 MiB through the actual controller stream. See
[AWS operations](design/aws-operations.md) and [ADR 0035](adr/0035-streamed-aws-downloads.md).
Maintained local CI and eleven intercepted SDK tests passed. The current owned
Host streaming adapter transferred 20 MiB in dedicated WSL without AWS access.
Real AWS remains SKIP for absent Host prerequisites. Guest CLI integration, native Windows
filesystem acceptance and AWS desktop decisions are still separate pending scope.

## Approved AWS S3 listing

Partial D3: trusted Host S3 listing is wired through the product CLI, controller,
shared Policy/approval/audit and optional AWS capability plugin. Account/principal
are rechecked using a frozen Host credential set. SDK region redirects, including
signing-region-only changes, are refused before transport. See
[AWS operations](design/aws-operations.md) for preparation, exact scope and limits.
Focused race tests and ordinary controller review/config revocation checks passed.
Eight real-SDK tests with synthetic credentials and intercepted HTTP transport
passed, including a signing-region-only redirect regression. Downloads are now
implemented above; guest CLI integration and real AWS/desktop acceptance remain
planned. Intercepted SDK tests do not prove real AWS acceptance.

The current Host adapter also passed a dedicated WSL execution with missing AWS
prerequisites, returning not-configured without AWS access. Actual AWS remains SKIP
because that owned Host has no AWS CLI, botocore or AWS config.

## Completed OCI copy recovery

Implemented: the canonical resource lifecycle persists a positive completion
receipt before Host restoration. A retry of ordinary setup/Host entry or
Environment creation can restore and publish that exact completed copy without
recopying. Ownership, journal and restart guards remain checked; missing receipts
and unknown completion remain recovery-required. See
[ADR 0033](adr/0033-completed-copy-recovery.md).

Focused race regressions passed reopened-state recovery, retained reservations,
malformed/foreign journal refusal and idempotent restoration. Dedicated WSL recovery/image acceptance passed in 355.73 s (project
`haco-area-d6ad75cf558f514e`): an injected post-completion resume failure retained
the guard, reopened state recovered the same copy without recopying, both runtime
images executed offline and all owned resources were cleaned. Installed CLI
recovery and unknown-completion recovery remain unproven.
At `ae0c245`, all four GHA workflows passed. Its Btrfs job 101949881165 actually
ran Docker/nerdctl image identity/offline execution and complete cleanup (109.79 s),
not only synthetic data checks. Private-registry acceptance was SKIP because the
workflow-dispatch-only job did not run.


## Docker Store roots and real Host-image acceptance

Implemented: Store attachment configures Docker's managed persistent/transient
roots, preserves unrelated options and refuses existing default data, conflicting
roots, active Docker units and unsafe configuration files. Matching configuration
is reused. Focused race regressions and vet passed. The owned-area E2E now builds
actual Docker/nerdctl images in Host and checks copied identities and offline
execution; the maintained Btrfs GHA job enables it.

The first real run built and executed both Host images, then failed Docker image
inspection in the copy (336.37 s): Host used `/var/lib/hacocoon-oci/docker` but the
copy used `/var/lib/docker`. The inspected fixture was fully removed through
canonical resource deletion (11.75 s). The corrected fresh run passed (376.60 s; project
`haco-area-3147b9dd5920fb2c`): both locally built images retained their identities
and executed offline after COW; deleting copy images left Host images usable.
Btrfs ancestry, bidirectional area writes/deletion and complete fixture cleanup
also passed. This covers Docker 28.5.2/vfs and nerdctl 2.3.5/containerd 2.3.3/native
in owned provider fixtures, not every driver/version or installed CLI recreation.
At f3f5557, all four GHA workflows passed, including the real image-copy and
completed-copy recovery extension. The initial failure stays recorded.

At `470a2b8`, all four GHA workflows passed, including Windows run 34188963290.
Actual Remote-SSH editor read/write, terminal execution, trusted review, Host
customization cleanup and installed notification subscription passed. Human
fresh-toast decision and VPN/NRPT remained explicitly SKIP; successful workflow
completion does not turn those into passes.


## Owned Host nesting

Implemented: the maintained OCI setup enables nested runtimes only after positive
ownership, unprivileged instance, profile, source and lifecycle verification.
The setting is persistent and reused on repeated setup. See
[ADR 0032](adr/0032-owned-host-nested-runtime.md). Focused race tests and vet passed, as did composition/OCI lifecycle tests.
Dedicated WSL `Hacocoon-Review-6771f2f` passed real nesting/reuse, nested mount
namespace, Host pause/COW/resume, independent writes/deletion and complete owned
fixture cleanup (58.56 s; project `haco-area-e8168370b7f8d3f8`). No fixture setting
remains. An initial PowerShell argument parsing failure occurred before test
execution and was corrected. The later actual Docker/nerdctl result is recorded above; the namespace-only
result did not itself prove image recovery.


## Interactive desktop Environment selection

Implemented: `haco open` and `haco ssh setup` offer an Environment/Workspace list
when multiple Environments exist in an interactive terminal. A sole Environment
still needs no input. Blank input cancels before setup; noninteractive ambiguity
requires an explicit name without reading stdin. The SSH client rechecks the
selected creation/runtime identity, Workspace and access mode during setup.

Component regressions and real-PTY product-process tests passed selection and
cancellation with a private fixture controller and temporary desktop directory.
They do not prove a multi-Environment Windows/VS Code connection. Existing
single-Environment editor evidence remains separate; new GUI acceptance is pending.

At `711005a`, GHA test/Ubuntu/Incus passed. Windows run 34185304876 passed
installation/restart/reinstallation and actual SSH setup/reuse, but failed the
Host customization cleanup and notification activity check. Repeated setup was
reproduced exhausting systemd's start limit. Healthy identical services now remain
running; changed executable revisions/configuration still restart. Twelve Python
regressions and eight consecutive real-systemd refreshes plus cleanup passed.
An intermediate edit had a Python indentation error and was corrected before
these successful runs. Full Windows acceptance subsequently passed at `470a2b8`
as recorded above, with human toast/VPN scopes still skipped.

## Fresh notification service startup

Windows run 34181502807 failed at notification-service setup and skipped the
subsequent connection/native checks. A deterministic component defect was
reproduced on dedicated WSL: unconditional `reset-failed` rejects a new unloaded
unit. An attempted explicit load still failed because systemd can unload it again.
Startup now resets only an observed failed unit; unknown results fail closed.
The 11 Python regressions and real systemd fresh startup/refresh/owned cleanup
passed. These component checks alone did not prove installed resolution. The
full Windows path later passed at `470a2b8`; fresh human notification activation
remains unverified.

## Fresh Host OCI storage setup

Partial: ordinary `haco setup` creates and binds an owned source area for a fresh
Host, configures its containerd/Docker data roots and verifies repeat setup.
Existing data, symlinks and custom configuration are refused without migration;
failed preparation retains ownership. No new daily command or mandatory runtime
installation is added. Existing-data migration
and actual runtime recovery remain incomplete. Host nesting is covered by the owned-Host setup slice above.

Local dedicated Incus/WSL setup, repeat verification, area COW, independent writes
and deletion, and exact fixture cleanup passed (53.19 s). This used synthetic
area data, not Docker/nerdctl images. The earlier provider commit `29fd6d1` passed
GHA test, Ubuntu installer and Incus E2E; its Windows run 34181502807 failed at notification service setup; downstream acceptance was skipped.

The initial configuration regression failed on Python without `tomllib`. The final implementation avoids that dependency and passed focused race tests and the real provider E2E.

Maintained local CI passed all Go tests/vet, 11 WSL and 3 approval Python cases, and 27 JS cases. Documentation consistency and its seven regressions passed.

Copy initialization now repeats the managed Host source readiness and configuration
check immediately before journal/pause. Setup-time validation alone could copy an
obsolete area after a later layout change. Regressions reject absent readiness,
layout drift and failed/truncated verification without pausing or copying. The
canonical source/destination recovery reservation remains retained on failure.

## Host area copy provider

Partial: the Incus backend can pause an exact owned Host with its source-only data
volume, perform existing area-level COW, verify completion and resume. A durable
copy marker disables autostart and blocks ordinary Host entry during uncertainty.
Foreign/duplicate consumers, mismatched attachments, pre-existing pauses and
unconfirmed copy/resume/cleanup are refused. Component tests and local provider E2E passed. The new real
Incus E2E is wired into the existing Btrfs job and passed at `29fd6d1`.

Existing Host data migration, Docker/containerd application recovery and
operator recovery of interrupted copies are still missing. Pausing processes is
not graceful daemon shutdown. No image enumeration or export/import is used.
[Protocol and limits](adr/0031-host-oci-area-copy.md#provider-pause-and-restart-guard).


Local dedicated WSL acceptance passed the provider mechanism in project
`haco-area-23ef9c90488e244b`, including real pause/COW/resume, Btrfs parent UUID,
bidirectional write independence, source deletion and exact cleanup (39.15 s).
The first actual run reached the copy/independence checks but failed final project
deletion because its downloaded Base image remained. That exact fixture image,
project and pool were removed; the E2E now removes its recorded Base identity.
A preceding PowerShell invocation failed argument parsing before starting the test.
Neither failure is counted as a pass. This uses synthetic area data, not OCI
runtime images. Focused race tests/vet, workflow policy and docs passed.

The maintained local CI test entry passed Go tests/vet, 11 WSL and 3 approval Python regressions, and 27 JS tests. The final Host start/copy process-lock change is checked separately with focused regressions and a new provider E2E run.

Final provider code, including the cross-process lock and exact autostart restoration, passed the dedicated E2E in `haco-area-a7d74034ed65d7d4` (37.98 s). The owned image, Host, volumes, project, pool and private recovery catalog were cleaned up. Final focused race/vet also passed.

## Actual Host OCI area copying

B4 requires direct area-level Btrfs COW of the actual Host image storage, not
image selection or export/import reconstruction. The contrary inventory slice
was reverted. Existing independent volume copy is aligned; the missing actual
Host runtime compatibility and existing-data migration remain incomplete. Do not claim delivery from
synthetic storage tests. [Decision](adr/0031-host-oci-area-copy.md).

Windows `4bb8dad` run 34176272125 failed: native review required unavailable
Get-FileHash. `8d7a2ea` replaces that dependency with .NET SHA-256; PowerShell 5.1
component tests passed. Installed Windows acceptance after the fix is pending.
The former OCI focused tests passed, but were removed with that withdrawn slice.
Its broader local CI was canceled while still running after successful individual
Go package results; full Go-suite and CI completion were not established and is not evidence for the corrected area-copy implementation.


## Git and network approval parity

Implemented shared approval behavior is now covered by cross-capability CLI and
Policy/audit regression tests: one-shot decisions, all six saved choices, exact
target scope, reevaluation and same-name Environment recreation. Focused race tests
and vet passed. The initial test expected approval-denied for saved deny; corrected
expectation is policy-denied after reevaluation, identically for both providers.
No new real Git push or HTTPS connection was performed for this regression slice.
See [the shared contract](design/pending-approval-review.md#shared-git-and-network-decisions).


## Automatic desktop notification follow-up

Implemented in the working branch: Windows post-install enables an owned Host
notification service after registration; `-SkipDesktopReview` stops/disables it.
First start skips historical presentation, while existing cursors resume normally.
Companion publication uses verified atomic replacement so a running notifier does
not block updates. Unit parser, ownership, opt-out, from-now and race regressions passed.
Installed automatic service acceptance remains pending.
At `4bb8dad`, test, Ubuntu and Incus E2E passed. Windows failed; see the current correction above.


## Host notification acceptance and state safety

At `213fb2b`, Ubuntu installer run 34173412741 passed installed Host notification
subscription and listener cleanup (job 101898000285). The test workflow passed.
Incus run 34173412776 failed Host setup: its standalone CLI fixture builds no
notification companion, now required by setup. The fixture is updated to build
and verify its installed digest/ownership; the corrected Incus run 34174437698 at `6d516d3` passed.
Windows run 34173412761 remains in progress.

Native cursor storage now rejects linked/special files, bounds reads, pins the
owned parent, synchronizes atomic saves and locks one writer for the process
lifetime. Focused tests and vet passed, including separate-process exclusion and
parent replacement. Windows automatic startup is now implemented; fresh native decisions remain incomplete.


Real Host component acceptance also passed in Hacocoon-Review-6771f2f using the
new worktree notification binary in an owned temporary directory: existing
controller subscription without audit projection, public schema and listener
cleanup. The temporary executable was removed. Direct execution from the Windows
mount first failed the fixture's installed-mode assertion; the corrected temporary
0755 copy passed. This does not prove normal installer delivery or native activation.


Local release-provenance validation **failed** on the Ubuntu 22.04 validation distro: the installer requires Ubuntu 26.04 or newer. The same release-provenance check subsequently passed on the dedicated Ubuntu 26.04 distro with a command-scoped Git safe-directory setting (no persistent Git configuration change). The earlier 22.04 failure remains recorded; normal notification package acceptance is still pending.


## Ordinary Host notification follow-up

Implemented in the working branch: same-release notification provisioning,
controller-mode event subscription with no local fallback, and validated Windows
distribution identity projection. Focused Go tests and WSL identity regressions
passed. Installed Host subscription E2E is added to Windows and Ubuntu workflows;
its new packaged run is pending. Previous Physical Host evidence does not prove
this new ordinary Host path, and fresh human notification decisions remain unverified.



Read-only inspection also confirmed that installed haco-host has neither haco-notify
nor the audit file. The native evidence above is from the Physical Host. Completing
ordinary Host notification subscription remains a priority before claiming daily D2 usability.


## Windows notification review

Status: **implemented adapter slice; roadmap D2 remains partial**. Windows bundles
include a native helper, checksum and per-distribution user protocol registration.
Notifications open the existing approval console with only an exact request ID;
they never answer or expose a management endpoint. See [the contract](design/pending-approval-review.md)
and [ADR 0030](adr/0030-windows-notification-review.md).

Local testing in Hacocoon-Review-6771f2f passed registration, installed stale-request
and malformed-link refusal, Windows protocol launch of the exact helper, and a
notification-history entry with the expected URI. The helper SHA256 was
e79df7c870f6218440479ea0d833e3c3d398a2499fff0eae4cc6bfd902acfc0b.
Later notification delivery failed before PowerShell with system error 8 because
the native WSL executable registration was absent while interop remained enabled.
The installed canonical WSL setup restored the registration after its normal
validation; final delivery and exact notification-history checks passed. No /init
fallback or notification-owned binfmt mutation was added. The reason the registration
disappeared remains unconfirmed. The initial helper attempt failed before the trusted
Host finished starting. Maintained local CI, focused native tests, package checks,
PowerShell syntax, docs and GoReleaser validation passed.
Native visible-click/fresh-decision acceptance and Linux activation remain unverified.

The preceding 05c8206 passed all four GHA workflows: test 34166655131, Ubuntu
34166655270, Incus 34166655133 and Windows 34166655142. Windows explicitly passed
local review stale refusal, ordinary HTTPS saved-ask/allow/re-prompt denial,
actual VS Code, preview/Edge and doctor. These results do not cover the new native adapter.

The corrected observer at `05c8206` passed actual local VS Code 1.136.1 acceptance against installed `6771f2f`: Environment `win-ssh-33848c2759174f10`, Windows loopback port 40429, remote file read/write, remote terminal execution, local custom approval terminal and installed-controller stale-request refusal. Ordinary HTTPS saved-ask denial / one-shot allow / re-prompt denial passed again. The normal fixture finished with exit 0: temporary Policy, SSH connection, Environment, Workspace, keys and observer files were removed; listener absence and Windows connection refusal were verified. This is real local manual-SSH/Remote-SSH acceptance, not a fresh human approval through the UI or native toast activation.

At `5283705`, test 34165137831, Ubuntu 34165137686 and Incus 34165137697 passed. Windows 34165137705 failed at local approval review and project setup for the approval prerequisite. Remote editor/terminal, ordinary setup, preview/Edge and doctor passed.

The corrected snapshot installed successfully in the dedicated local WSL instance `Hacocoon-Review-6771f2f` (installed commit `6771f2f38f8c036a2fb16e8f9640377229b11c65`). Local Environment `win-ssh-d5dc5903cceb456a` passed Windows SSH on port 37713, actual VS Code 1.136.1 remote editor/terminal, and actual HTTPS saved-ask denial / one-shot allow / re-prompt denial through ordinary `haco approve`. Local approval-terminal acceptance failed; diagnostics and a regression for failure-path probe cleanup were added. Connection and Environment removal and listener refusal passed. The initial cleanup failed on retained observer files; exact owned files and empty directories were subsequently removed. The earlier existing-instance update was rejected by automatic approval review and not executed; the dedicated installation was separately approved.

At `6771f2f`, test 34163005164, Ubuntu 34163005175 and Incus 34163005206 passed.
Windows 34163005171 failed overall VS Code acceptance (the exact stage was not
reported) and approval Python preparation; preview/Edge and all four doctor checks
passed. Fixed stage reporting now separates editor timeout/remote checks/local review,
and prerequisite execution from clearing its recipe. No new UI success is claimed.

The local snapshot build and corrected dedicated installation passed; the first packaging invocation without the leading v failed before installation completed. See the current physical acceptance results above.

## VS Code trusted review

Status: **implemented repository slice; roadmap D2 remains partial**. The optional
UI extension opens ordinary approval in a local custom terminal from Review or the
command palette. It fixes executable routing and environment, rejects remote/web or
untrusted execution and requires the existing CLI answer. VSIX packaging needs no
npm download. Related 26 JS tests passed. The existing real VS Code GHA now includes
local terminal to installed controller stale-request refusal; its result is pending.
Native OS activation and fresh human review acceptance are not proven by this slice.
See [the contract](design/pending-approval-review.md) and [ADR 0029](adr/0029-local-desktop-approval-review.md).

At `0754280`, test 34161070477, Ubuntu 34161070466 and Incus 34161070522 passed.
Windows 34161070471 passed actual VS Code, preview/Edge and all four doctor checks.
Only approval acceptance failed, at Python prerequisite preparation before review;
cleanup succeeded. Fixed diagnostic categories now distinguish setup-unit/package/
DNS failures without raw output. The root cause is not established; this is FAIL,
not SKIP, and the new diagnostic rerun is pending.


The generated optional VSIX passed archive/manifest checks and installation into an
isolated profile of the actual local VS Code. This verifies packaging only, not a
fresh approval or local terminal/controller round trip.

## Pending approval review

Status: **implemented repository slice; roadmap D2 remains partial**.
haco approve selects a sole pending request automatically or offers a numbered choice.
The Standard queue bounds background approval waiting; the common private review API
also uses original Git prompts. One-shot decisions, six saved choices, cancellation,
expiry, duplicate submission, exact completion ownership, Policy changes, sanitized
failure receipts and real local Git helper integration passed related race/vet tests.
Windows notification activation is implemented; fresh native decision acceptance remains partial. See [pending review](design/pending-approval-review.md).

Installed GHA now includes ordinary configuration plus actual HTTPS saved-ask denial,
one-shot approval, re-prompt/denial and scoped cleanup. At `5ad8c3e`, Windows run
34159087435 failed in saved-ask denial, preview setup/open and doctor invocation.
Actual VS Code, SSH, configuration and project setup passed. Test 34159087438,
Ubuntu 34159087434 and Incus 34159087447 passed. Maintained local test/E2E and docs
also passed. The approval fixture now prepares Python through ordinary setup and
accepts its normal completion line; installed rerun of these corrections is pending.
Preview/doctor failures now include fixed phase and numeric metadata without raw output.

At f6d193b, test 34154746874, Ubuntu 34154746842 and Incus 34154746852 passed.
Windows 34154746844 passed actual VS Code, SSH, configuration, project setup and doctor,
but HTTP preview failed. Its exact cause remains unresolved.

A local installed `71dbb4f` rerun passed Windows native SSH, changed-host-key
refusal and cleanup on port 33105 (`win-ssh-67210d9ab7994c7d`). The scoped temporary
Policy, connection, Environment and Workspace were removed; the listener was absent
and Windows connections were refused afterward. Two earlier attempts failed because
the Host was stopped; the successful attempt kept the ordinary Host terminal open.
Automatic desktop setup was SKIP locally because that fixture uses a disposable GHA
profile. This is SSH evidence for the installed snapshot, not installed acceptance
of the new approval review or a new local VS Code run.

## Approval correlation

Status: **implemented groundwork; roadmap D2 remains partial**. Approval prompts,
trusted controller responses and pending Git proposals share the capability audit/result
request ID. This adds no command, approval authority or notification action endpoint.
See [interaction events](INTERACTION_EVENTS.md#approval-correlation).

At `2584ec6`, GHA test 34152700790, Ubuntu 34152700745 and Incus 34152700884
passed. Windows 34152700897 passed native SSH, actual VS Code, configuration round-trip
and project setup, but failed HTTP preview and Environment doctor. Their exact
causes remain unresolved. Earlier installed successes are separate evidence,
not success for those failed probes.

## Approval configuration editing

Status: **implemented repository slice; roadmap D remains partial**.
`haco config` and optional `--edit` / `--file` use the same Policy as ordinary
saved approvals. Revision checks and the common private writer prevent concurrent
saves from being overwritten. Audit records only operation/revision metadata.
Focused test/race/vet and maintained CLI/controller E2E passed; installed acceptance
of configuration round-trip passed at 2584ec6. See [configuration](reference/configuration.md).

Local `71dbb4f` installation passed six Host checks. Configuration read/replace,
receipt, file revision and audit were verified without changing default deny or
the original eight rules. JSON-view equality failed on an empty saved_decisions
array omitted at write time. Snapshot canonicalization and its component/CLI
regression now pass; installed configuration round-trip with the correction passed at 2584ec6.
See [the exact observation](reference/configuration.md#local-installed-observation).

At `71dbb4f`, all four GHA workflows passed: test 34151576434, Ubuntu 34151576429,
Incus 34151576447 and Windows 34151576493. Windows included ordinary configuration
round-trip, actual VS Code, project setup, Edge preview and Environment doctor.

A separate local `71dbb4f` journey used `haco config --file` to add and later
remove only four temporary Ubuntu archive rules for `preview-71dbb4f`.
Ordinary project setup started a loopback HTTP server. Windows received its
exact Workspace marker at port 36059; preview reuse, close/refusal and local
runtime/Workspace/DNS doctor checks passed. No SSH connection was prepared in
this local probe. The marker, recipe, Environment and listener were removed,
and default deny/eight original rules/zero saved choices were verified. Existing
Workspace `git-save-eb16300` was retained. Earlier intermittent preview/doctor
failures were not reproduced here; their original causes remain unresolved.

At `729f008`, GHA test 34149690153, Ubuntu 34149690280 and Incus 34149690192
passed. Windows 34149690178 passed DNS, desktop SSH/resume, actual VS Code and
project setup. Preview and Environment doctor failed; the modified fixture ran
both and kept the final job failed. Their exact causes remain unresolved.

## Reusable ordinary Git approvals

Status: **implemented repository slice; D1/D2 remain partial**. Existing Git
approve/deny commands accept optional --save for Environment/global allow, deny
and ask. Pending output separates current exact commits from provider-declared
reusable scope. Only OIDs and operation ID are wildcarded; repository/remote/ref
and fast-forward update kind stay fixed. Attribute-name matching remains exact.
Saved responses require durable persistence and audit, and unsupported peers fail
closed. Real local Git helper tests cover subsequent commits, ask, deny and
history-rewrite refusal. Installed Windows/WSL acceptance at `eb16300b6700`
passed ordinary SSH, saved ask approval, GitHub push, subsequent re-prompt and
denial with unchanged remote. See the exact commits and retained resources in
[managed Git acceptance](reference/managed-repository-workflow.md#installed-saved-approval-acceptance)
and [ADR 0026](adr/0026-reusable-git-approval-scope.md).

At `eb16300`, GHA test 34146281274, Ubuntu 34146281278 and Incus 34146281289
passed. Windows 34146281264 failed: DNS and ordinary SSH/resume passed, then
VS Code did not complete within ten minutes. Setup/preview/doctor later in that
job were not run. This remains a failure, not a SKIP or current editor acceptance.
The fixture now continues independent setup, preview and doctor checks after an
editor failure, retaining each failure for the final job result. PowerShell
syntax was checked locally; changed-fixture GHA acceptance is pending.

At 953d1e5, all four GHA workflows passed, including the corrected orchestrator
and crash-recovery fixtures. This does not establish acceptance of later changes.

## Policy-bound name resolution

Status: **partial roadmap C3**. Installed Standard mode now automatically installs
the guest loopback DNS service during canonical Environment creation and resume.
The service notifies readiness after binding UDP/TCP; failed installation or
startup prevents successful creation/resume. Bare controller mode keeps the
component optional. No new user command, nameserver argument or allow rule is
required for provisioning. Lookup still requires its own Policy permission.

The existing guarded listener binds requests to persisted source identity and
uses Capability Policy/audit before the Physical Host resolver. Connection
authority remains separate. A Windows GHA fixture now compares ordinary
getaddrinfo results across Windows, WSL, trusted Host and Environment and checks
default DNS denial; both passed at `c05528a` in Windows run 34132173483. VPN/NRPT, propagation
after DNS changes and restart remain unverified. See [name resolution](design/name-resolution.md).


At `72096d8`, local test/vet/docs/e2e and affected-package race checks passed.
GHA test and Incus passed; Ubuntu run 34121278716 and Windows run 34121278578
failed while configuring the Environment DNS service, before the new DNS fixture
could run. A separate local probe on the older installed substrate started the
same DNS unit successfully; it does not reproduce or explain the GHA failure.
The probe was canonically deleted and its empty Workspace removed. The adapter
now returns only an allowlisted failure phase and numeric service exit status
to diagnose the installer failure without exposing arbitrary guest output.
At `a1d084b`, the bounded guest-manager wait passed Ubuntu installer, Incus
and test workflows. Its Windows run was cancelled at the job time limit.
At `c05528a`, test, Ubuntu installer and Incus workflows passed. Windows run
34132173483 passed DNS equality/default denial and real VS Code connection,
then failed before project setup: CRLF in the PowerShell-generated Bash script
made `set` exit 2. The harness now normalizes both setup and preview scripts
to LF. Setup, preview and Environment doctor acceptance await the rerun.

C4 [project setup](design/project-setup.md) now implements explicit Workspace
recipes through `haco setup --script <path> <environment>`, replay and clear.
Host recipes retain their existing behavior. Target identity is checked before
start and under the execution lifecycle lock; script bytes travel through
bounded stdin. Relevant race tests passed. Installed GHA coverage was added but
failed in the harness before setup execution at `c05528a`; package installation and actual cancellation
cleanup acceptance remain unverified.

## Current desktop-development checkpoint

Status: **partial roadmap C**. Product commands provide desktop SSH setup,
`haco open [--client vscode|ssh] [environment]`, retained resume and readable
Environment target discovery. Saved Host setup recipes are now implemented through
`haco setup --script <path>`, replay and `--clear-script`; installed Windows GHA
passed at bcc1baf. The temporary-run product CLI and real-Incus acceptance passed at 4adfe19. Broader C1 selection, C3–C5 and later roadmap stages remain incomplete.

All four GHA workflows passed at `4f1f512`. The Windows job proved actual VS Code
1.136.1 Remote-SSH document read/write, terminal execution and owned probe cleanup
through ordinary `haco open`. It uses a disposable portable profile with the Linux
platform choice saved and Workspace trust prompts disabled; this does not cover
normal desktop prompts or every Windows/VPN configuration. Earlier `703ec76`
failed executable discovery; `506c38f` advanced past launch but timed out waiting
for editor acceptance. Those failed runs are distinct from the successful rerun.

The ordinary local installer last upgraded the existing distribution to `8752431`
(v0.33, build `2026-09-07T06:44:17Z`). Doctor passed six checks and preserved
`stage-b-git-dev` and its Workspace. Installer ZIP SHA-256:
`c2c5b720643d98e586996e2d2413d1af196d764331e2b160a5bda647c76946a9`.
This installation predates the current DNS changes. Local acceptance passed on
2026-09-07 after the user authorized temporary package-egress rules for
`desktop-8752431`: native Windows OpenSSH and VS Code 1.136.1 Remote-SSH verified
the Workspace marker, editor read/write, remote terminal execution and probe
cleanup. The editor used a separate local profile and explicit Remote-SSH URI;
this is not local acceptance of ordinary `haco open`, which GHA covers separately.
The four temporary rules were removed (remaining count zero), SSH connection
`ssh-39493` revoked, disposable Windows key deleted and test Environment stopped.
The user's `stage-b-git-dev` remained stopped. Earlier local resume failed because
WSL restart removed its volatile source guard; recreation used canonical deletion
and creation. Its old /tmp Workspace was also absent, so a fresh test directory
was created. These failures are separate from subsequent connection success.
The missing-guard resume fix now has regression coverage; actual reboot acceptance
of that fix remains pending.

## Independent persistent Store copies

Status: **implemented storage slice; full revised B4 remains partial**. Based on
main `3b2d0b6` (merged PR #481), `haco plugin oci store create dev --from shared`
creates an independent copy of an unleased Store without adding a command group.
The canonical catalog reserves the exact source until provider completion and
verified publication. Failure retains ownership and requires manual recovery;
automated interrupted-copy recovery is not implemented. See the
[Store contract](design/persistent-oci-store.md#independent-offline-copies).

Local real Incus 6.0.5-8 / WSL / Btrfs acceptance passed with synthetic data:
Btrfs parent UUID matched the original, writes were independent in both
directions, source deletion retained the copy, and test volumes/project/pool
were removed. This does not establish OCI image/runtime, installed CLI or
trusted Host publication acceptance. The initial E2E fixture failed because
its project did not exist before pool inspection; the fixture was corrected
and the rerun passed. An initial RPC regression exposed invalid-argument
classification as internal error; the handler was corrected.

A/B prior work is preserved: PR #481 merged as `3b2d0b6`; its final `0b79cac`
[test](https://github.com/SLktEx/Hacocoon/actions/runs/34081379821),
[Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34081379810),
[Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34081379802) and
[Windows installer](https://github.com/SLktEx/Hacocoon/actions/runs/34081379870)
workflows completed successfully. These are prior-commit results, not CI for
this copy change. The new storage test is wired into existing Incus CI.

SKIP for this slice: trusted Host image acquisition/publication and actual
containerd/Docker consumption of copied images (that full product path is not
yet implemented); VS Code UI development (no IDE exercise in this slice);
Windows packaged acceptance of this change (installed product remains the
previous B candidate); a new real Git push (no Git/auth/ref change here, and
copy acceptance uses no repository). Existing B push OIDs below are historical
verified results, not a new push. C-G implementation is still planned according
to the updated [roadmap](status/architecture-and-roadmap.md#user-facing-development-order).


## Validation of the Store copy slice

Local checks passed: maintained CI `test` (Go tests/vet, installer components and
JavaScript), `e2e` command/capability/Git/orchestrator assertions, docs/workflow
policy, and the real-Incus synthetic COW test. `forwarding` initially failed for
missing passwordless sudo; running the same isolated-namespace test as root in
the development WSL passed. E2E assertions passed, but its first temporary Go
module-cache cleanup emitted permission errors; this is a cleanup failure,
not a product assertion failure. Re-running the command E2E with the existing
`GOMODCACHE` explicitly selected passed without cleanup errors. The earlier
temporary path was independently confirmed absent.

The first full `race` run failed in the existing CONNECT upstream-prefix shutdown
test. The serving proxy now synchronously owns/closes CONNECT upstreams and
rejects a dial completing after shutdown before any write. The proxy package
passed 100 race repetitions; the full maintained `race` gate then passed.
See the [egress contract](EGRESS_AUTHORIZATION.md). No authorization policy or
user command changed in this fix.

The monolithic `bash tools/ci-local.sh` stopped at release-config because the
Ubuntu development WSL has no `pwsh`; that run **failed**. Standalone Windows installer component tests passed in Windows PowerShell.
Release provenance initially failed the development Ubuntu 22.04 minimum-OS
check. Running its fixture-only checks on Ubuntu 26.04 passed provenance and
installer package contracts after trusting this exact Windows-owned worktree
for that process only. GoReleaser configuration validation passed. Full release
archive builds and fresh package installation remain **SKIP** for this slice.

New GitHub Actions execution is **SKIP / publication blocked**: automatic approval
review rejected pushing the implementation branch to `SLktEx/Hacocoon` because
explicit destination authorization was present only for push testing to
`SLktEx/Hacocoon-test`. No source push or PR was created. The new COW test is
ready in the existing Incus workflow for a later authorized publication; prior
B CI success does not verify this revision.

## Incus startup PID protection

Status: **implemented; repository regressions and hosted Ubuntu/WSL package
acceptance passed**. The common Ubuntu/WSL installer installs a root-only Incus ExecStartPre
guard that archives prior-namespace dnsmasq/proxy PID records before daemon
startup. Current-namespace records and resource data are retained. See the
[Host contract](design/trusted-host.md#wsl-default-entry) and
[ADR 0013](adr/0013-incus-pid-record-boot-identity.md).

Nineteen component cases cover reused PIDs, WSL/native boots, service restart,
initialization, concurrency, interrupted retirement and unsafe metadata. Installer,
package and Windows driver regressions pass. The Windows package gate also checks
marker renewal and archived dnsmasq records after restart. Same-namespace helper
PID reuse and optional device families remain upstream scope.

On `1b2d6ae`, the [Windows package gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931616)
passed installation, ordinary WSL termination/re-entry, marker renewal, retained
dnsmasq records, installer rerun and installed egress enforcement. The
[Ubuntu package gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931562),
[real Incus gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931583) and
[repository CI](https://github.com/SLktEx/Hacocoon/actions/runs/34051931607) also passed.
The optional authenticated-private-registry job was skipped; these results do
not add private-registry acceptance. Final revision and merge status are in
[PR #480](https://github.com/SLktEx/Hacocoon/pull/480).

The hosted v0.28 run did not update the local installation at that time. The
revised fresh Stage B acceptance below includes this guard from main.

## WSL startup failure investigation — 2026-09-07

Status: **historical investigation; startup failure reproduced, stale-PID cause
strongly supported**. Cross-namespace prevention is implemented above. This investigation used installed product
`029ff08e34c98e075b7b0b3d3a7fc7f639e89323` and Ubuntu package
`incus 6.0.5-8` on the local WSL 2.7.12 host. It does not establish the
cause on another Windows account or machine.

At 02:32:21 JST, the preceding successful startup assigned PID 424 to the
`haco-host0` dnsmasq process. After ordinary WSL shutdown and startup,
at 02:33:13 the kernel trace captured `kill(424, SIGKILL)` targeting
`libuv-worker`, followed by Incus main PID 248 exiting with signal 9.
The trace's target kernel PID was 9468; 424 is the caller-namespace ID,
not the same numbering space. An independent process listing confirms
that these libuv workers are threads of the Incus daemon. No OOM event
was established.

The upstream [v6.0.5 dnsmasq cleanup](https://github.com/lxc/incus/blob/a87f49a2491fa3a0e74896c1f2322bd356c59ddc/internal/server/dnsmasq/dnsmasq.go)
imports the saved `dnsmasq.pid` and calls
[`Process.Stop`](https://github.com/lxc/incus/blob/a87f49a2491fa3a0e74896c1f2322bd356c59ddc/shared/subprocess/proc.go).
It checks numeric PID existence and kills it without validating boot,
process start time or executable identity. This is consistent with a
previous dnsmasq PID being reused as an Incus thread ID. The exact
user-space call stack of the failing signal was not captured; the PID,
target thread, previous dnsmasq identity and failure time were correlated.
Normal forkproxy/helper SIGKILL events were also observed and must not be
mistaken for daemon failure.

A separate test using only a test-owned child process confirmed that
`pidfd_open(worker_tid)` returns ENOENT, while numeric signal-0 succeeds
and SIGKILL to that TID terminates the entire process. This verifies the
OS-level failure mechanism, not an applied Incus fix or an end-to-end
regression for the upstream package.

After Incus dies during startup, its 600-second `waitready` start-post
process can remain, blocking the controller's `After=incus.service`.
The product login then reaches its two-minute deadline. The login client
also discards the last transport error, so the timeout alone cannot
identify this cause or distinguish socket permission failures.

All temporary kernel traces and uprobes were removed. One bounded
observation included an explicit Incus service restart; later installed
`haco doctor` passed all six checks. This recovery does not close the
intermittent failure. No provider binary, PID file, storage, network or
installed service configuration was patched. Correct ownership validation
belongs in the provider's process lifecycle; automatically deleting its
PID files from Core or increasing the login timeout is not a root fix.

## Current Stage B revision

Status: **implemented; revised local packaged acceptance passed**. Main baseline
was `0665ba9`; this is branch-candidate evidence, not a published release or a
claim that unrelated open PRs have landed. Current scope is native WSL interop,
actual multi-drive projection, Persistent OCI Store and Windows native OpenSSH.
`switch-base` is disabled at the public CLI and deferred to Stage D+; it does not
block A-C. Historical code/ADR/evidence remain. See the
[roadmap](status/architecture-and-roadmap.md#current-stage-b-scope),
[OCI Store](design/persistent-oci-store.md) and
[Windows SSH procedure](reference/windows-environment-ssh.md).

**Installed candidate:** `c86c43e4f2702c5fccadd91f542d82bd8b733706`, checkpoint
`v0.29`, snapshot `0.27.0-SNAPSHOT-c86c43e`, built 2026-09-07 10:38:30 JST.
Windows ZIP SHA-256:
`39205fea38aa7b38f8474edb6f957363b3565a22a00e6957d64ba542217566db`.
Later commits refine tests and documentation; the accepted installed product is
this exact candidate. Configuration: Windows 10.0.26200.9278, WSL 2.7.12,
kernel `6.18.33.2-microsoft-standard-WSL2`, Ubuntu 26.04, Incus `6.0.5-8`,
Incus-owned Btrfs pool `haco-local-default`. Default Base revision:
`sha256:297ce79fb308c09126222dd6e64c260003c5d1e1ea1ce46ea43e80a419941636`.

**Fresh path:** verified that only Ubuntu and Ubuntu-24.04 existed and Hacocoon
was absent, extracted the branch-built ZIP, ran its ordinary `install-windows.bat`,
entered `wsl -d Hacocoon`, and passed all six doctor checks. The maintained
Windows installer gate then passed WSL termination/re-entry, retained Host data,
the same BAT rerun and cold doctor. Additional native checks passed Host-only
`incus restart haco-host --project hacocoon` without setup in between, actual
`wsl --shutdown` followed by normal entry, and a later ordinary `haco setup`
rerun/doctor. No source binary, mount, PATH, socket or service repair was injected
into final-package acceptance. Windows/WSL features already existed; this is a
fresh Hacocoon distribution, not an OS reinstall or Windows OS reboot test.

| Step | Observed result on the final candidate |
|---|---|
| B1 | Actual DrvFs inventory detected C and Q, projected at `/mnt/c` and `/mnt/q`. On both NTFS drives, Windows-created files were read from Host, Host-created files were read from Windows, and spaced paths/arguments worked. Files created before WSL removal remained readable after fresh installation and both restart paths. These are the same Windows filesystems, not copies inside a disposable Environment. |
| B1 native execution | Absolute `cmd.exe` ran without an explicit `/init`, followed by `cmd.exe /c ver`, `powershell.exe -NoProfile -NonInteractive`, Windows-PATH `where.exe`/`findstr.exe`, stdout `hello`, stderr marker and exit 23. The shell received only WSL-converted PATH entries under detected Windows drives. OCI and SSH Environment create/delete did not break interop in the already-open Host session. |
| Former B3 | Packaged `haco env switch-base` returned exit 2 with `currently disabled` and Stage D guidance before any controller mutation. Base selection during ordinary Environment creation remains supported. |
| B4 | Two Stores had different provider identities. Creating/attaching Store A, rejecting its deletion while leased, pulling BusyBox, building/running a local image, deleting the Environment, inspecting the retained Store, attaching it to a new Environment, reusing both images without registry access, and observing cached build steps passed. A separate Store started empty while the Workspace remained. Environment deletion kept Stores; explicit Store deletion removed them and subsequent inspection returned not found. |
| B5 | Windows standard `System32/OpenSSH/ssh.exe` connected through Windows 127.0.0.1:22229, WSL Physical Host loopback and the Incus loopback proxy to Environment sshd. Generated config used strict checking and a dedicated trusted-provider host-key pin. `/workspace` was usable; a mismatched key failed before execution. Disconnect/delete removed the WSL listener and Windows reconnection failed. Windows may report timeout rather than ECONNREFUSED after removal. |
| B2/A/B6 | One `stage-b-git-dev` Environment contained two independent `.git` directories with no shared commondir or alternates. Windows SSH fetch/pull, edit, commit and product-helper approved push passed for both. After disconnect/stop, status named the Environment, Workspace, Base and retained state. A later ordinary WSL entry verified the Environment remained stopped and its untracked Workspace note remained intact. |

OCI versions were containerd `2.2.2-0ubuntu1.1`, nerdctl `2.3.5` and BuildKit
`0.33.0`. Runtime binaries and daemon proxy settings were installed in each
Environment through its exact package/download Policy. `pull --unpack=false`
stored content; `run --snapshotter native` prepared snapshots. Built images and
BuildKit cache stayed in the Store, while process/socket state stayed in `/run`.
Before and after reattachment, BusyBox image ID was
`sha256:c6348fa86ba0fb2108c9334f5fe913ddc6d853313e655891f133a0127c30099f`
and the local image ID was
`sha256:475bcd7010f2b330b1b82f7a43a911baeb6be801dfd1d2fb2d6b7b498a99c7bb`.
Docker Store compatibility is **not** accepted by these results. Earlier Docker
image-distribution evidence below is historical.

All Git result writes targeted only `https://github.com/SLktEx/Hacocoon-test.git`.
The two registrations intentionally use this one authorized remote with separate
branches; different remote URLs remain covered by local real-Git regression.
Both proposals were inspected for repository, Environment, URL, ref, operation,
old OID `f4ff6e33588a7183b0c7d3db2f4c2214a527678f` and fixed new OID before
`haco git approve`; independent Windows `git ls-remote` then matched:

| Registration / branch | Verified remote commit |
|---|---|
| `stage-b-first` / `codex/stage-b-20260907-first` | `7f9f9ecaaae1cc332c3a42d9724eeddbb9701f4d` |
| `stage-b-second` / `codex/stage-b-20260907-second` | `98168553a91e20f2f97b0658bfd305ab4ed488e6` |

Btrfs independently reported source UUIDs `8dbe4029-79b1-5f46-96d1-522b9cf9fd6a`
and `d84cae46-8f22-2144-99c8-d7d6ac5a6c9f` as the respective parent UUIDs of
Workspace volumes `a558d778-1ac5-1047-9c04-d72568d530ff` and
`8b7b06f4-8b40-7d4d-92e8-4643074ca769`. This proves the observed independent COW
relationship, not performance. `managed:stage-b-both` and its stopped Environment
are retained for the user.

**Boundary/validation:** Environment checks found no `/init`, WSL socket,
Windows drives or Windows executable access. Windows client private keys were
created and removed only on Windows; only public keys entered Hacocoon. Git
credentials remained in trusted Host's root-only standard gh store. The existing
Windows Git token was transferred via stdin after official `gh api` identity
and test-repository push-permission checks; no token was logged or put in an
Environment. `gh auth login --with-token` initially rejected missing additional
OAuth scopes; no broader scopes were granted (see the
[GitHub CLI contract](https://cli.github.com/manual/gh_auth_login)). This was
manual credential setup, not a new product broker.

The installed egress workload passed allowed HTTPS, denied-proxy 403, blocked
direct TCP and absent management sockets. Maintained local CI phases passed:
docs, workflow policy, Go tests/vet and JavaScript tests, race, E2E, and isolated
kernel forwarding. Release/package, native interop, lifecycle ownership and
guest-systemd-readiness regressions passed. Release checks requiring supported
systemd ran on Ubuntu 26.04; Windows installer/BAT components ran on real
PowerShell 7/5.1. The developer Ubuntu 22.04 could not run the monolithic release
phase unchanged; its supported-platform components were run separately. These
are local results, not a claim of new hosted CI or private-registry acceptance.

Reproducible drivers are
[`windows-installer-user-path-e2e.py`](../tools/windows-installer-user-path-e2e.py),
[`windows-native-access-e2e.py`](../tools/windows-native-access-e2e.py),
[`test_windows_environment_ssh.ps1`](../tools/test_windows_environment_ssh.ps1),
[`test_persistent_oci_store.py`](../tools/test_persistent_oci_store.py) and
[`installed-egress-check`](../tools/installed-egress-check/main.go).
Local proof logs are under `bin/stage-b-c86-*`, especially fresh-package-gate,
persistent-oci, native-ssh-cleanup, approved-git-standard-credentials,
two-repository-cow and retained-workspace. Logs and credential material are not
part of the source archive.

Manual Git authentication/Policy, SSH key/config/pin handling and optional OCI
runtime/proxy installation remain. Wider Windows/image/runtime compatibility,
hotplug, external removal of a live WSL binfmt handler, interrupted operations,
upgrades and generic recovery remain unverified. The earlier handler disappearance
was observed but its trigger was not established; normal setup/entry reconciles
only a missing handler through WSL's own generated service. Stage D+ automation,
switch-base reconsideration, registry/broker infrastructure, concurrent Store
sharing and live migration are listed in [follow-ups](status/development-follow-ups.md).

## Historical second-stage workflow (superseded scope)

The following is preserved commit-bound evidence for the earlier request.
Former B3 and delivery-only B4 are historical, not current requirements.


Status: **implemented; B1–B6 locally accepted** on the Windows/WSL configuration
below. Docker and nerdctl both passed one-way distribution and independent
container execution. The selected B5/B6 improvements and the affected A workflow
also passed. See the [workflow](reference/managed-repository-workflow.md),
[OCI contract](design/persistent-oci-store.md) and
[follow-ups](status/development-follow-ups.md).

| Step | Implemented behavior and observed result |
|---|---|
| B1 | Explicit trusted-Host setup discovers existing DrvFs drives. PowerShell received a separate spaced argument, preserved stdout/stderr and exit 23; user-owned `/mnt/c` reads/writes passed. Rechecked on final installed `029ff08`. Only C is mounted; extra-drive parsing has regression coverage, additional-drive real-host acceptance is not executed. Environments have no Windows devices, `/init` or interop environment. |
| B2 | Packaged `087e7e2`: one `b-dev` Environment mounts independent Btrfs copies at `/workspace/b-first` and `/workspace/b-second`, with separate `.git`, no alternates/commondir. SSH fetch/pull, edit, commit and fixed-content approval push passed for each repository. GitHub returned `be34f60c2c3d1ab5761e821fbdaada5e4d5802dc` and `b834ee67dbc8f5e37e73656f13872d42ceda40f3` on the respective test branches. Two different remote URLs also pass local real-Git regression. |
| B3 | Packaged `3747bae`: `haco base list` and `env switch-base` moved the same managed Workspace from Ubuntu 26.04 to 24.04. All 54 repository files, including `.git`, had identical hashes after switching. Unpushed commits `bce47b9` / `6d5fc53`, modified tracked files and untracked notes survived. Reconnected Git/SSH, pinned the new host key, and edited over SSH on Ubuntu 24.04.4. |
| B4 | Packaged `029ff08`: `haco plugin oci distribute --runtime <runtime> --image hacocoon-b4:smoke b-dev` passed separately for Docker and nerdctl. Each guest container started over ordinary SSH, changed its file, then stopped; the corresponding Host container remained running with its original file. Unit/component regressions cover both command families, failed export, input/size bounds and fixed instance-local sockets. |
| B5 | Packaged `029ff08`: `haco env ssh-config b-dev` generated OpenSSH configuration and standard `ssh -F ... haco-b-dev` connected successfully, removing manual host/port/user transcription. Existing Incus 6.0.5's unsupported `config show --format` was replaced with its JSON query API and regression coverage. |
| B6 | Packaged `029ff08`: `haco env status` clearly displayed Environment, state, Workspace, access and Base. Stopped status explicitly said the Workspace is retained. `--json` preserves machine-readable output. |

**Test target:** all external writes used only
`https://github.com/SLktEx/Hacocoon-test.git`, branches
`codex/stage-b-b-first-20260906` and `codex/stage-b-b-second-20260906`.
The two real-host registrations use this same authorized URL with separate
branches; different remote URLs were exercised only in repository regression.

Btrfs independently reported source UUIDs
`411102dc-d913-264a-96a0-b09d079eb898` / `58ccd7df-d8df-3444-98b4-67b35d85018e`
as the parent UUIDs of the respective Workspace volumes
`49eff338-40d8-244b-9276-e35952b475b2` / `a23fadbc-77af-be4a-b7a9-f9829e96e613`.
This verifies the observed COW relationship, not performance.

**Final installed candidate:** `029ff08e34c98e075b7b0b3d3a7fc7f639e89323`,
checkpoint `v0.28`, snapshot `0.27.0-SNAPSHOT-029ff08`, built
`2026-09-06T10:25:40Z`. ZIP SHA-256:
`20f308cb5bcccfdaef1f0c76914bdae65834c957afd6c446fd0effdda26717fe`.
Each candidate was built from its branch commit and applied through its ordinary
Windows BAT to the existing Hacocoon WSL distribution. No installer-specific
product override or internal state repair was used. Fresh installation was not
repeated. The retained A configuration is Windows 26200.9278 / WSL 2.7.12 /
Incus 6.0.5 / Incus-owned Btrfs `haco-local-default`.

**Bases:** B3 moved from revision
`sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c`
to Ubuntu 24.04 revision
`sha256:f38ca805517f5b6e301f33b0f44523386c5a050847564c1233e586106b31dbc9`.
Later explicit Ubuntu 26.04 creation resolved
`sha256:297ce79fb308c09126222dd6e64c260003c5d1e1ea1ce46ea43e80a419941636`;
the earlier Environment's recorded revision was unchanged.

**A regression on final candidate:** ordinary single-repository `b-a-work` /
`b-a-dev` creation with explicit Base, generated SSH configuration, fetch and
fast-forward pull from `f4ff6e3` to `be34f60`, Python compilation/assertion, commit,
denial (remote unchanged), then approval push all passed. GitHub independently
returned `145fd7fce49a5a8771e39e7b142d47aa49c910c3` on the first test branch.
Disconnect and graceful stop succeeded; all 28 files including dirty/untracked
work and Git state had identical hashes before/after stop. The canonical lease
and Incus volume remain. Inner/outer clients matched the controller build;
all six doctor checks passed. The original A Workspace/Environment remains
preserved. After OCI setup, SSH/helper fetch still passed in both repositories,
allowed proxy access passed, direct external TCP remained denied, and Windows
interop/controller paths remained absent from the guest. Ordinary `env stop
b-dev` retained all 57 Workspace file hashes and the collection ownership;
both Host containers remained running. `b-a-dev` and `b-dev` are stopped.

**B4 configuration and result (2026-09-06):** nesting was explicitly enabled on
owned, unprivileged `haco-host` and `haco-b-dev`; existing devices and network
guards were retained. Independent Ubuntu `docker.io` installations provided
Docker 29.1.3 on both sides and containerd 2.2.2 (Host) / 2.2.1 (guest).
Official minimal nerdctl 2.3.5 archive SHA-256 was
`de3206aeb7cbd5f20f5fb1f55c1e3bf2db1be567812a8a3f5e65eba2488347ee`.
No full bundle, privileged mode, AppArmor disabling or shared runtime device was
needed. The image contained only Ubuntu `busybox-static`, its shell symlink and
fixed `/data/message`; image ID was
`sha256:9bafa1f9ed06b9fcc33ef5b6674ef3c4d79ae819b7724d5b228923712112b46f`.
Both product transfers reported 1,183,232 bytes and archive SHA-256
`a2ea9ac81b39572d424bd2b63461ac659c2b0a4c327ccb963e110f08ed553c57`.
Both used `--network none`. Docker guest content became `guest-only`, nerdctl
guest content became `nerd-guest-only`; both Host copies stayed `host-original`.
See the [reproducible procedure](design/persistent-oci-store.md).

**Repository validation:** maintained `ci-local.sh docs`, `workflow-policy`,
`test` (Go tests/vet and JS), `race` and `e2e` passed after B5/B6. Narrow lifecycle,
Git, collection-mount, OCI and SSH-configuration regressions passed while
iterating. GoReleaser check/build and installer archive checksums passed.
The isolated Linux kernel forwarding job also passed. Local validation used
Go 1.27.1. Hosted CI, including release-config and installer/provider jobs, is
recorded on [PR #473](https://github.com/SLktEx/Hacocoon/pull/473).
Broad real-host runtime/network matrices are not claimed.

**Manual operations and acceptance limits:** B1 requires the recorded Physical
Host script; repeat after interop socket changes. Authentication and narrow
Policy remain trusted-side setup. SSH private key and host-key pinning remain
client-owned; use the controller's Physical Host/Windows loopback, since a
different WSL distribution may have its own loopback namespace. SSH package
downloads still need credential-free proxy exports (existing #469). Base switch
discards root filesystem/packages and requires Git/SSH reconnection. B4 requires
explicit nesting/runtime setup in each instance, repeated after Base replacement.
The earlier execution-review refusal was resolved on the resumed B-completion
request; the exact-instance setup and known test-image transfers above executed
successfully. Additional Windows drives were unavailable; wider Windows/image
compatibility, interrupted operation and general recovery remain unverified.

## Managed repository WSL workflow — 2026-09-06

**Implemented; A1–A6 accepted on the local Windows/WSL configuration below.** The v0.27 candidate adds
product CLI repository registration, independent Incus Btrfs Workspace copies,
controller-backed Environment creation/SSH, a Git-only remote helper and
graceful stop retaining Workspace ownership. Authenticated Git executes inside
trusted `haco-host`; Policy, approval, state and Incus authority stay in the
Physical Host controller. Ordinary helper fetch, conflict-free pull, deny and
approval pinned to old/new OIDs pass a real-Git local integration regression,
including a local branch change during approval. Local repository tests and
the separately observed packaged journey are distinguished below. See the
[workflow](reference/managed-repository-workflow.md) and
[ownership decision](adr/0008-managed-repository-workspaces.md).

**Packaged acceptance:** `7a4d1227c95642f27cb118c3d20d2cd554e8be32`, version
`0.27.0-SNAPSHOT-7a4d122`, built `2026-09-06T07:57:54Z`. Windows ZIP SHA-256:
`0468c8f95c5b431c5d4160aead860deb152ed8d8e381b321c6b85b2f650d1a80`.
Windows build `26200.9278`, WSL `2.7.12.0`, kernel `6.18.33.2-2`, installed
Ubuntu 26.04, Incus `6.0.5`, Incus-owned Btrfs pool `haco-local-default`.
The ordinary packaged BAT applied to the existing Hacocoon distribution and
exited 0 after all six doctor checks. No CI product overrides or internal
resource repairs were used. Fresh absent-distro installation of this candidate
was **not** repeated; prior installer acceptance below remains separate.

| Step | Observed result |
|---|---|
| Entry and controller | Ordinary `wsl -d Hacocoon` entered trusted `haco-host`; inner and outer product clients returned the same build and `poc-dev` Environment |
| Repository and COW | Registered `poc` from `https://github.com/SLktEx/Hacocoon-test.git`, branch `codex/wsl-poc-20260906`; independent `poc-work2` copy mounted at `/workspace` in `poc-dev` |
| Default Base | `haco/ubuntu-26.04`, revision `sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c` |
| Development | Standard OpenSSH with a client-owned key and pinned server key; Python byte-compilation, two unittest cases and a commit succeeded. Own `.git`, no `commondir`/alternates, no Host gh credential file or management socket; trusted source worktree remained unchanged |
| Fetch/pull | Ordinary helper fetch and `pull --ff-only` advanced the same Workspace from `f4ff6e3` to the remotely prepared `19caa79e123b981227d1c0b58783c7a6af80e930` |
| Denial | `haco git deny` rejected the pending push; remote stayed at `19caa79` |
| Approval | Proposal fixed `19caa79` → `c18cbb8e202cecc0d6c80b29a8cd700dc1c0558f`, the registered URL/ref and operation. `haco git approve` let ordinary `git push` finish 0; GitHub independently returned exactly `c18cbb8`. Audit recorded denial, approval and successful completion |
| Finish | SSH commands exited, `env disconnect poc-dev ssh-2222` revoked the endpoint, `env stop poc-dev` succeeded and inner/outer status reported `stopped`. Reconnection was refused |
| Retention | The exact canonical Workspace lease and custom volume remained. Unpushed HEAD `5650953d591fc6294a0db8db5f71a408e7917555`, modified `greeting.py`, untracked notes and the branch-ref file had identical SHA-256 before/after stop; remote remained `c18cbb8` |

Btrfs source UUID `760d7b7e-0e0e-7f4c-9f88-7303ad96f55c` matches the parent UUID
of Workspace `f3326bfe-8cb0-684e-a3a3-d437dd3b817e`. This establishes the observed
Incus COW relationship, not a performance measurement. The first candidate
`c116307` registered the source but failed Environment write access because it
dropped Incus ID-map bookkeeping while copying. A focused provider regression
and `7a4d122` fix preserve that bookkeeping. The failed `poc-work` copy was kept;
the accepted journey used a fresh ordinary `workspace create` from the same
registered source, with no chown or privileged-Environment workaround.

**Manual setup retained:** install `git gh` and authenticate gh inside trusted
`haco-host`; create exact Git/Ubuntu egress Policy rules on the Physical Host;
provide the SSH public key and pin its host key; export the credential-free
Standard proxy URL inside the SSH shell before apt. Existing authorized gh
credentials were transferred by stdin to the trusted Host only. Pre-application
Host HTTP/HTTPS attempts timed out; the normal BAT's setup then restored passing
readiness without separate repair. The initial failure's cause is unconfirmed.

**Repository validation:** maintained `ci-local.sh docs`, `workflow-policy`,
`test` (all Go tests, vet, JS syntax/tests), `race` and `e2e` passed. The E2E
expectation that product `env` was unavailable was updated, and the local E2E
runner used existing Go caches outside its temporary HOME. The Incus ID-map fix
passed focused race regressions; GoReleaser check/build and package checksums
passed. The complete release-config/forwarding jobs and a new hosted CI run are
not claimed by those local checks.

**Deferred / unverified:** SSH proxy setup is [#469](https://github.com/SLktEx/Hacocoon/issues/469);
interrupted approvals, unknown push results and retry are [#470](https://github.com/SLktEx/Hacocoon/issues/470).
The next requested B1 is trusted `haco-host` Windows exe execution and available
WSL drive mounts, under the existing [#275](https://github.com/SLktEx/Hacocoon/issues/275)
boundary; no automatic Environment exposure. B2/B3 multiple repositories/Base
switching, larger packs, other auth methods, force/multiple refs, LFS/submodules,
generic recovery/cleanup, resume UX and a wider host matrix remain outside this
acceptance. The retained test Environment is stopped; both Workspace copies and
the test branch are intentionally preserved. B/C implementation is not part of
this checkpoint's acceptance work.

The historical M1 observations below remain tied to their original builds.

## WSL delivery update — 2026-09-06

This candidate branch implements the following WSL slice; the requested WSL M0–M1 scope is **implemented and accepted**. Refreshed main `e8974ef` includes #441/#442/#453/#456 and #458/#459. Its two later release-bootstrap/revert commits have no net file changes; merge `b58f82c` has the same product tree as the accepted `c749ff9`. A merged PR or an older green run is not acceptance of later product changes.

- **Implemented storage:** Incus-owned Btrfs is the only distribution/runtime path. External `driver`/`source` attachments and uncertain inspections fail closed. Desired policy is `compress=zstd:3,noatime,nodiscard`. The [read-only mount diagnostic](design/btrfs-storage-layout.md#read-only-mount-diagnostics) separates configuration, verified live application and `pending` application. Backing device/inode, one full-image loop association and the Btrfs root mount must agree. Unknown, malformed or changing observations cannot pass. Hacocoon adds no separate image/loop/mount lifecycle or diagnostic repair.
- **Implemented installer and trusted Host:** the default account is non-root `hacocoon` with a locked password; `-InteractiveUserSetup` is optional. Current reruns preserve account identity/password state and write no sudo policy. Controller-owned `haco setup` provisions the owned trusted host and narrow endpoint; the common installer now requires all product doctor checks before completion. Fresh hosts have explicit devices without inherited profiles. The owned `haco-host0` bridge provides trusted infrastructure DNS/DHCP/NAT, and Docker forwarding is restricted to that bridge and established replies. See [bootstrap](WINDOWS_WSL_BOOTSTRAP.md), [trusted Host](design/trusted-host.md) and ADRs [0004](adr/0004-wsl-installer-authority.md), [0005](adr/0005-trusted-host-network-ownership.md), [0006](adr/0006-controller-owned-host-setup.md).
- **Partial product CLI:** new `haco` provides help/version, `setup`, `doctor` and the controller-backed WSL login alias without invoking `hacoq`. The Physical Host owns controller state, Policy, providers and Incus authority. There is no guest controller or Incus daemon. [Diagnostics](design/controller-client-transport.md#host-diagnostics) return six ordered checks with bounded failure/pending actions. The controller wait and read-only guest DNS/route startup wait do not retry failed external checks or repair resources. The broader lifecycle/Base/SSH CLI migration remains separate; the #456 controller adapter is reusable.
- **Implemented Standard proxy lifecycle:** the installed controller owns the fixed proxy listener and verifies shared guards before binding. Control/proxy shutdown is coupled, including hijacked CONNECT tunnels. The daemon has no ambient approval provider; exact allows stay audited and require-approval fails closed. Same-PID listener and unmanaged-source refusal are distinct from allowed Environment traffic. See [ADR 0007](adr/0007-controller-owned-standard-egress.md).
- **Repository validation — `c749ff9`:** focused race/vet tests, pending CLI/API regressions, nine Windows assertions, five installer shell tests, shell syntax and documentation checks passed. The maintained `ci-local.sh test` passed the full shuffled Go suite, vet, two JavaScript syntax checks and five notification tests. An earlier local vet attempt included downloaded research sources under `bin/`; after renaming those observations to `.txt`, the complete entry point passed. These tools supplied no product environment overrides or installed-resource repair.
- **Planned Seed retirement:** Seed code remains with [Base/optional OCI dependencies](design/oci-seed-and-cow.md). Base selection and optional Plugins remain supported architectural boundaries.
- **Implemented registration continuation, Windows package accepted:** failed WSL inventory cannot become assumed absence, and native creation success requires exact registration readback. A failed creation/readback saves an advisory stage/options record for a manual current-BAT rerun; it grants no authority and is never executed. Explicit exit 3010 propagates as restart-required, while exit 0 without registration remains incomplete with a conditional reboot action. PowerShell 5.1 component tests and actual BAT exit-propagation tests passed. Windows feature installation and OS reboot are not accepted by those tests. See [bootstrap continuation](WINDOWS_WSL_BOOTSTRAP.md#interrupted-registration-and-windows-restart).

Packaged acceptance is bound to **`c749ff9033b33c3526e108f60ce2009638075152`**:

| Environment | Observed acceptance |
|---|---|
| [Windows gate](https://github.com/SLktEx/Hacocoon/actions/runs/34008408570) | Exact cached BAT fresh creation, ordinary entry, stop/reentry, same-version rerun, cold doctor, build identity, retention, proxy listener ownership and unmanaged-source 403 passed |
| [Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34008411207) | Packaged ordinary-user installation and trusted-host checks passed |
| [Incus gates](https://github.com/SLktEx/Hacocoon/actions/runs/34008410296) | Standalone, owned Btrfs, authenticated private registry and Core jobs all passed |
| Current Windows host | Unchanged ZIP application and same-version BAT rerun exited 0 after all six readiness checks; normal entry, both clients' full build identities, retained UUID/file/account/sudo policy, Btrfs state and proxy checks passed. Doctor after confirmed distro stop exited 0 in 51.906 seconds |

The local ZIP is `0.26.1-SNAPSHOT-c749ff9`, built at `2026-09-06T03:12:38Z`, with SHA-256 `f638379fb293cf249f32ef46b5576b95906ff775bc2f00f96ae3ed602724d3f9`. Fresh Windows acceptance means an absent Hacocoon distribution on the runner's current-WSL substrate, not disabled Windows features or a Windows OS reboot. Local retained data is the trusted-host sentinel and baseline, not proof of uncommitted/untracked/unpushed Workspace retention.

**Unresolved startup failure:** an ordinary entry on `42e2fb3` saw Incus main PID 282 receive SIGKILL at 11:33:30 JST, leaving the standard 600-second start-post waiter and controller dependency blocked. The signal source is unconfirmed; available kernel records did not establish OOM. Incus's standard automatic restart began at 11:43:16 without manual service/mount repair, and later entry/retention passed. The guest DNS/DHCP startup race has a separate fix and acceptance. Neither that fix nor the later `c749ff9` successes establish the SIGKILL source or the cause of the earlier independent WSL exit-9 observation.

**Registration package acceptance — `4df465a71aedcdc70c28b543220b79b2465808ab`:** [Windows run 34010791925](https://github.com/SLktEx/Hacocoon/actions/runs/34010791925), job `101426135649`, passed the exact fresh cached BAT, ordinary entry, restart, same-version rerun, retained data, six doctor checks and both PowerShell/BAT regressions. Local PS5.1 native inventory/argument tests, package/provenance checks, `ci-local.sh docs` / `workflow-policy`, and native documentation consistency passed. The provenance test's first Ubuntu 22.04 attempt correctly failed the 26.04 baseline; it passed on the supported substrate without changing product requirements. Local ZIP SHA-256 is `439dfc8a0a4dab5ef4adf05f1b1ed9b3e02883a5009b66dca7513c528d0d3105`, version `0.26.1-SNAPSHOT-4df465a`, build `2026-09-06T04:10:02Z`. It was built and checksummed, but not installed locally again: the current local installation remains accepted `c749ff9`, while CI supplies the changed fresh-registration/rerun coverage.
**Current M1 scope:** the latest user direction excludes actual Windows OS reboot implementation/acceptance and further continuation work. Keep verification proportional to a concrete change or failure, and put additional maintained regressions in CI. Existing successful checks need no repeat without new evidence. The required installed Environment allowed-proxy/denied-direct acceptance through the existing controller/provider boundary has now passed. The unexplained startup incident remains documented; the later scoped signal observation did not identify its cause. Broader diagnostic features, firewall-order matrices, CLI/SSH development and Workspace retention remain follow-up work, not expanded completion conditions.

**Environment source correction:** the Windows gate on `f373cfc` passed the exact BAT journey but its allowed HTTPS probe returned proxy 403. The persisted source resolver compared provider-local refs with the routed refs produced by Environment creation. It now matches both provider and native ref using the canonical router decoder. A focused regression reproduces the failure with the actual Base router's creation result and refuses another provider's identical native ref.

**M1 acceptance — `81c0d160722b96864daa8d6f5f3b9ea86423ff48`:** [Windows run 34013409969](https://github.com/SLktEx/Hacocoon/actions/runs/34013409969), job `101432997324`, passed fresh cached BAT installation, ordinary entry, stop/restart, same-version rerun, retained trusted-host data and six doctor checks. The installed-controller Environment check also passed certificate-verified allowed HTTPS, unapproved-hostname 403, direct TCP refusal, management-socket absence and controller cleanup. CI checked PR merge commit `9049df39f8000e32103b6a2f3939ea3d14fc5ffe`; its entire tree was verified identical to candidate `81c0d16`. Local egress, Environment router, composition and Standard proxy tests passed after the routed-reference regression failed on the previous code. Documentation consistency passed.

The new local ZIP is `0.26.1-SNAPSHOT-81c0d16`, built at `2026-09-06T05:12:08Z`, SHA-256 `4938622b994a66b71d5647086819db63e7ee7a7a8ea1189e3b2ad964ccb69c6b`. GoReleaser packaging and all distribution checksums passed. This ZIP has not been reinstalled on the current Windows host, whose installed version remains `c749ff9`; candidate Windows acceptance above is from CI. Actual Windows OS reboot remains outside scope.

**Historical next item:** M2 Environment creation is implemented in the v0.27 candidate above; packaged journey acceptance is tracked separately.

The historical checkpoint tables below retain their original milestone context.

Status date: 2026-08-31, after cloud deferral, the Base/OCI CLI split, Docker compatibility lifecycle integration, the OCI Seed Builder repository slices including credential-free managed-Environment harvest, the client-neutral interaction-event contract, the reusable client-adapter contract, domain-aware Standard egress enforcement, Incus-owned Btrfs rootfs pool integration, default Incus-managed Btrfs `zstd:3` compression, browser/native/VS Code notification clients, phased real-Incus CI acceptance on Ubuntu 26.04, the shared structured logging foundation, ordinary-user real-Incus storage acceptance, and the persistent trusted `haco-host` / default WSL entry slice.

This file reports **current code reality**, not desired architecture. Hacocoon is pre-1.0; implementation does not imply API stability, production support, or real-host acceptance beyond explicitly named acceptance checks.

The current milestone position is **v0.57**. Milestones are lightweight development checkpoints: v0.17 still has acceptance work, but that partial status does not block later implemented checkpoints such as v0.18-v0.26.

| Area | Current repository reality | Milestone |
|---|---|---:|
| Secure Workspace Runtime / Workspace leases | Incus-backed Environment lifecycle, canonical Workspace identity, RO/RW leases and recovery are implemented | v0.1-v0.2 |
| Client access | status, loopback forwarding and SSH preparation/revocation are implemented | v0.3 |
| Policy / Capability | fail-closed policy, approval and audit are implemented | v0.4 |
| Git / GitHub push | privileged push is brokered on the trusted Host without exporting reusable Host credentials | v0.5 |
| Agent / orchestrator integration | `haco run`, machine output and external events are implemented; orchestration remains outside Core | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` projects capability audit records into minimized stable event types with deterministic IDs, resumable cursors, bounded batches, recovery/attention flags, and public corruption errors; observation never authorizes or executes a capability | v0.6 / cross-cutting |
| Environment routing | the provider-neutral routing seam remains implemented; **cloud implementation is currently deferred** and concrete EC2/AWS/EBS code is absent from the active tree | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` exposes client-owned DTOs for exact Environment ensure/reuse, status, loopback SSH/TCP connections, revoke/delete, `/workspace` discovery, and `pkg/interaction` batches; ordinary `haco ssh` is the non-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`, per-agent binding and `haco-agent-host` foundations are implemented | v0.8-v0.10 |
| Base lifecycle | provider-neutral Base identity and `haco base list` / `haco base inspect` / `create --base` are implemented | v0.11 |
| Resource budgets | CPU, memory, PID and root-storage budgets are modeled and Incus finite limits are enforced or rejected | v0.12 |
| Managed sandbox network | managed `haco-sandbox0`, proxy-only ACL transport guard and `haco-sandbox` profile are created/verified; bridge DNS is disabled while DHCP remains; drift fails closed | v0.13 / cross-cutting |
| Git fetch plugin | `haco plugin git fetch <environment>` uses trusted Host Git/GitHub authority including `gh auth git-credential` for HTTPS private repositories | v0.14 |
| OCI usage telemetry / Seed recommendation | `haco plugin oci seed sample` records image identities and `recommend` ranks immutable identities; deterministic top 10% eligible recommendations become `auto_promote=true` | v0.15 implemented |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` records deletion tombstones; exact immutable identities can be explicitly re-enabled without silently undoing broader deletion state | v0.16 implemented |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build` / `current`, per-Base pinning, conservative GC/recovery, trusted Host acquisition, credential-free exact-image harvest from explicitly marked running managed Environments, offline no-NIC build, immutable publication/current pointer, exact-parent resolution, and pre-build interrupted-builder recovery are implemented; real-host/authenticated-registry/COW acceptance remains pending | v0.17 partial |
| Docker compatibility | `haco plugin oci docker status/prepare` validates a Base-provided genuine Docker profile, verifies pinned systemd units, refuses active vendor-daemon takeover, and enables Environment-local socket activation without making Docker a Core requirement | v0.18 implemented |
| Domain-aware egress authorization | Core `network.egress/connect` authority, Standard HTTP/HTTPS proxy, Host-side DNS pinning, private-address rejection, CONNECT/SNI validation, trusted Incus source-IP mapping and `haco egress serve` are implemented; real supported-Incus acceptance remains host-dependent | v0.19 implemented |
| Managed Btrfs rootfs storage | local composition lazily asks Incus to create the `haco-local-default` loop-backed Btrfs pool with `size=128GiB` and routes Hacocoon-owned Base/Tooling/Seed/Environment/trusted-host rootfs paths to that pool | v0.20 implemented |
| Managed Btrfs transparent compression | default Incus pool creation requests `compress=zstd:3`; `compress-force` and `autodefrag` are not desired defaults, while filesystem mount lifecycle remains Incus-owned | v0.21 implemented |
| Interaction notification clients | `haco-notify` provides loopback interaction delivery for browser and native OS notification flows, and the optional VS Code notification extension consumes the same client-neutral interaction stream; replay/dedup behavior is covered by tests | v0.22 implemented |
| Real Incus E2E acceptance | GitHub-hosted Ubuntu 26.04 first verifies standalone real Incus system-container behavior and then runs Hacocoon Core lifecycle E2E on a fresh runner; the phased gate covers systemd/exec, networking, hotplug, storage/snapshot behavior, diagnostics, and guarded cleanup | v0.23 implemented |
| Structured logging | shared `log/slog` foundation, INFO-default text/JSON output, Environment lifecycle operation fields, sanitized DEBUG Host-command tracing, egress authorization tracing, and defense-in-depth secret redaction are implemented across maintained executables | v0.24 implemented |
| Incus-owned Btrfs storage acceptance | the actual ordinary-user `haco` binary is exercised against real Incus; acceptance verifies lazy pool creation, sparse Incus backing image, loop attachment, Btrfs mount, zstd policy, writable Workspace flow, pool reuse, and guarded cleanup | v0.25 implemented |
| Trusted `haco-host` / default WSL entry | local Incus runtime can ensure and shell into a persistent trusted logical `haco-host`; exact ownership markers and reserved-name collision refusal protect the boundary, managed storage is used, WSL interactive entry targets `haco-host` by default while Physical Host root remains an explicit recovery path, and raw Incus control is not exposed into the trusted Host | v0.26 implemented |
| OCI plugin boundary | containerd/nerdctl/Docker-dependent behavior lives under optional `modules/plugin/oci`; `HACO_PLUGIN_OCI=nerdctl|docker` opts in, and Core remains valid when unset | cross-cutting |
| Optional Local OCI Registry | Registry/proxy is optional and not required for ordinary direct upstream pulls or Seed construction | unversioned optional / deferred |

## Domain-aware egress boundary

Ordinary HTTP/HTTPS egress is enforced through the Standard proxy rather than by DNS-to-IP ACL approximation. The Incus NIC remains default deny and allows only TCP to the managed bridge gateway on the Standard proxy port. The bridge keeps DHCP but disables its dnsmasq DNS listener with `raw.dnsmasq=port=0`; unmanaged DNS or ACL configuration fails closed.

The managed profile provides HTTP(S) proxy discovery to Hacocoon Environments. The proxy derives Environment identity from trusted Incus source-IP state, routes each hostname/port/protocol request through the existing Policy / Approval / Capability / audit path, resolves DNS only on the Host after authorization, pins the public answer set per connection, and validates HTTPS CONNECT against ClientHello SNI before forwarding TLS bytes. `haco egress serve` is the foreground trusted-Host launch path so the current stdio approval provider remains usable. See [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md).

## Notification clients

v0.22 turns the client-neutral interaction stream into user-visible notification adapters without moving approval authority into the client. `haco-notify` exposes the loopback bridge used by the browser and native notification paths, while `clients/vscode-notify` provides the optional VS Code consumer. Cursor persistence, replay, deduplication, corruption handling, browser behavior, and VS Code behavior are covered by repository tests. Reading or displaying a notification remains observational only and never approves or executes a Capability. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Real Incus E2E acceptance

v0.23 records a support-confidence checkpoint rather than a new Core API. GitHub Actions on Ubuntu 26.04 proves the Incus substrate independently before exercising Hacocoon Core. The phased standalone stage verifies real system containers, systemd/exec behavior, networking, device hotplug, storage/snapshot behavior, diagnostics and exact cleanup; the dependent Core stage then runs the Hacocoon lifecycle against real Incus on a fresh runner. This distinguishes an Incus substrate failure from a Hacocoon regression and prevents fake-only E2E from being treated as sufficient acceptance.

## Structured logging

v0.24 makes structured logging a named milestone. Maintained executables configure one shared `log/slog` root from `HACO_LOG_LEVEL` and `HACO_LOG_FORMAT`; INFO/text is the default and JSON is available without changing stdout command results. Environment create/exec/shell/delete operations carry stable `operation`, `environment_id`, duration, result/error fields through context. The trusted Host runner adds sanitized DEBUG command metadata and classifies Incus/network/storage/Git/OCI commands without automatically logging subprocess stdout or stderr. Network egress authorization adds normalized target/protocol and request correlation at DEBUG.

The shared handler redacts known password/token/API-key, authorization/cookie, credential-bearing URL, and secret-assignment patterns as defense in depth, including at DEBUG. Call sites still must omit arbitrary headers, environments, configuration objects, private keys, request bodies, and untrusted output. ERROR ownership remains at the operation/reporting boundary; lower Host/provider layers use DEBUG diagnostics rather than duplicating ERROR. See [`reference/logging.md`](reference/logging.md).

## Trusted `haco-host` / WSL entry

v0.26 introduces a persistent trusted logical Host on the local Incus path. `haco host ensure` creates or reconciles `haco-host`, and `haco host shell` enters it after ensure. Hacocoon marks exact ownership, rejects name collisions with non-owned instances, uses the managed storage path, and keeps the raw Incus control socket outside `haco-host`. The WSL login shim makes normal interactive distro entry target `haco-host`; explicit Physical Host root entry remains the recovery escape hatch.

Real Incus acceptance covers trusted-host creation, ownership, idempotent ensure, stopped-state recovery, managed-storage behavior, and control-socket non-exposure. Real Windows/WSL interactive-login acceptance is still host-dependent. The current slice establishes lifecycle/default-entry behavior; broader Git/OCI/credential/control-channel migration remains follow-up work. See [`design/trusted-host.md`](design/trusted-host.md) and [`WINDOWS_WSL_BOOTSTRAP.md`](WINDOWS_WSL_BOOTSTRAP.md).

## Client adapter boundary

`pkg/clientadapter` is the reusable adapter-facing contract for VS Code-independent clients. Exported signatures use package-owned DTOs and public error sentinels rather than `internal/core` types. The adapter can ensure/reuse an Environment only when the canonical Host Workspace and requested access mode match exactly, exposes the in-guest Workspace as `/workspace`, reconciles connection metadata, and composes the public `pkg/interaction` event contract.

SSH preparation accepts public-key material only. Clients retain their private keys and IDE configuration. Returned/reconciled SSH and TCP connections are revalidated as loopback-only; incompatible provider output is rejected and newly-created invalid connections are revoked or surfaced as recovery-required when cleanup cannot be proven. The existing `haco create` + `haco ssh` + ordinary `ssh` flow is the non-VS-Code proof. See [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md).

## Client interaction boundary

`pkg/interaction` is the reusable client-facing event contract. It reads the existing trusted capability audit stream and exposes only stable, presentation-safe fields: schema/event/request identity, UTC time, event kind, Environment/capability/action labels, attention/recovery flags, a closed failure code, and the next resume cursor.

Raw capability resources, authority attributes, opaque parameters, provider output, approval tokens, credentials, and free-form audit reasons are not part of the client schema. Browser, VS Code, code-server, JetBrains, and future adapters may independently observe/deduplicate these events; reading an event has no side effect and never substitutes for the trusted Policy/Capability approval or execution boundary. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Core/plugin boundary

With `HACO_PLUGIN_OCI` unset, Hacocoon Core must not require or probe for containerd, nerdctl, Docker CLI, Docker Engine, or a local OCI Registry. Base identity remains a Core/provider-neutral concept under `haco base ...`; OCI workload tooling lives under `haco plugin oci ...`.

The project-maintained OCI plugin profile may use containerd + nerdctl, and the Docker driver may provide genuine Docker compatibility. Neither choice defines a mandatory Hacocoon Core runtime.

## OCI Seed / storage

v0.17 has repository slices for build/publish, operations hardening, and credential-free managed-Environment harvest. The implemented path is trusted Host acquisition/cache -> offline no-NIC Seed Builder -> immutable Seed revision/current pointer -> exact-parent resolution -> normal Incus/storage-driver clone. One writable `/var/lib/containerd` must never be shared across Environments.

Explicit per-Base pins are persisted as immutable OCI identities. Deletion tombstones override recommendations and existing pins until the exact immutable identity is explicitly re-enabled. `haco plugin oci seed recover` reconciles exact Hacocoon temporary builders and then performs conservative GC; `haco plugin oci seed build` invokes recovery before a new build while holding the Seed build lock. GC does not manipulate Incus-owned Btrfs internals and retains current, in-use, instance-base, or externally aliased images. Deletion state is re-checked after publication so an operator deletion racing a long build cannot silently become current.

For an exact immutable identity already present in an explicitly marked running Hacocoon-managed Environment, Seed acquisition can copy a temporary `nerdctl save` OCI archive into the trusted Host cache and then delete the guest archive. It does not copy registry credentials, credential-helper output, workspace data, arbitrary Environment files, or live containerd state. Legacy/unmarked Environments are not inspected; failed or unavailable harvest falls back to the existing trusted Host pull path.

v0.20 makes local rootfs storage explicit without giving Hacocoon its own block lifecycle. Before an Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, local composition lazily ensures `haco-local-default` through Incus. Incus creates and owns the sparse backing file, loop device, Btrfs filesystem, and mount; Base, Tooling, Seed, trusted-host, and Environment rootfs volumes, snapshots, and clones share that Incus-owned Btrfs pool while Host Workspaces remain bind-mounted outside it.

v0.21 makes transparent compression the default creation policy: Hacocoon asks Incus for `btrfs.mount_options=compress=zstd:3`, does not request `compress-force`, and keeps `autodefrag` out of the desired default. Incus remains the mount lifecycle owner, and Hacocoon does not rewrite existing extents merely to recompress them.

v0.25 is the real ordinary-user acceptance checkpoint for that storage path. Dedicated GitHub-hosted Ubuntu 26.04 CI runs the actual `haco` binary against real Incus, verifies lazy `haco-local-default` creation, confirms the Incus-owned sparse backing image and loop attachment, checks the live Btrfs mount and zstd policy, exercises writable `haco create` / `exec` / `delete` / `run`, proves pool reuse, and verifies guarded cleanup. See [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md).

Local Registry is not a prerequisite and has no reserved milestone. Remaining storage acceptance includes authenticated/private-registry combinations using Host-owned credentials without leakage, physical Btrfs compression ratio and CPU-cost measurement, COW/compaction behavior, broader real-host failure injection, Windows/WSL behavior, and supported-host verification beyond the automated GitHub-hosted Ubuntu 26.04 CLI lifecycle.

## Docker compatibility

v0.18 is implemented at the repository gate. This code landed while the feature was temporarily numbered v0.17 and is reclassified without rollback. `HACO_PLUGIN_OCI=docker` exposes `haco plugin oci docker status <environment>` and `prepare <environment>`. `prepare` does not install packages or mount Host sockets: it requires the selected Base/Seed to provide Docker CLI, dockerd, containerd, systemd, the docker group, and the Hacocoon-pinned socket/service units. It fails closed on unit drift or an already-active vendor Docker daemon instead of silently taking it over.

Real Incus/systemd acceptance remains host-dependent and is tracked separately from repository implementation status.

## Cloud status

v0.7 retains the provider-neutral Environment routing seam because that architecture remains useful. The former concrete EC2/AWS/EBS implementation was intentionally removed while the local/provider contracts are still moving. **Cloud implementation is currently deferred** and must not be described as active or accepted.

## Acceptance gaps

Repository tests do not substitute for all real-host acceptance. v0.23 proves a phased real-Incus substrate plus Core lifecycle on GitHub-hosted Ubuntu 26.04, v0.25 additionally proves ordinary-user Incus-owned Btrfs CLI behavior, and v0.26 proves trusted-host lifecycle/control-socket isolation on real Incus. Real Incus networking/resource behavior beyond those paths—including proxy-only bridge ACL/dnsmasq behavior—Windows/WSL + VS Code and interactive `haco-host` entry, private-registry credentials, Docker compatibility, physical Btrfs compression/COW/compaction behavior, broader storage failure injection, desktop notification delivery, and future cloud adapters remain environment-dependent. Partial acceptance in an earlier milestone does not prevent later minor checkpoints from advancing.

## Resume retained Environments

Status: **implemented command and component slice; roadmap C/E remain partial**.
`haco env start <name>` resumes the existing runtime without extra required flags.
Active lease identity and Incus network isolation are checked before start;
create/start/stop/delete serialize across controller processes on Linux/WSL.
See [ADR 0016](adr/0016-resume-owned-environments.md).
Full local CI test and race phases passed. Independent real Incus 6.0.5-8 /
WSL acceptance passed: stop/start, repeated start, root filesystem and Workspace
contents, unchanged persisted lease and canonical deletion. The initial fixture
failed only its timestamp comparison (JSON removes Go's monotonic clock); comparing
persisted leases fixed it and the rerun passed. Both test runtimes were removed;
the existing user Environment remained stopped. The existing Incus E2E also now
includes trusted Host product stop/start and retained Workspace assertions;
that installed-product flow passed GHA at `f8517ba`. SSH setup automation and reconstruction of
missing guards after Host reboot remain unimplemented.

## SSH host public key through the ordinary API

Status: **implemented protocol slice; automatic SSH setup remains planned**.
Incus SSH preparation returns a structurally validated Ed25519 `host_public_key`
through the ordinary controller response. Native clients can pin it without a
separate administrator Incus command. Invalid key data revokes the managed key
and proxy; cleanup failure is recovery-required. Public adapters revalidate the
optional key. Related race tests passed; the Windows native acceptance script
now consumes this field; the installed Windows flow passed GHA at `f8517ba`.

User clarification: development-source pushes go to a branch in Hacocoon and
then a PR; Git push feature tests remain restricted to Hacocoon-test. PR #482
publishes the v0.30/v0.31 and SSH public-key slices at `f8517ba`. The earlier
publication rejection is resolved by explicit source-push authorization.
The required B4 default is now explicit: automatic OCI image publication/COW
copy at Environment creation with an optional opt-out. The ready-source copy slice
is implemented below; Host image publication remains planned. See the owning Store contract.

## Automatic Workspace Store initialization

Status: **implemented ready-source copy/reuse and opt-out; full B4 remains partial**.
Ordinary Environment creation now invokes the optional OCI default resolver.
A ready source-only `oci-source:host` is copied to a Store durably bound to the
Workspace; recreation reuses it. `--no-oci` skips this initialization. Without
a publication, non-OCI creation remains usable. Source-only direct attachment,
wrong-Workspace reuse, partial publication/copy and silent empty fallback are
rejected. No Host Docker/nerdctl image producer exists yet; this does not complete
automatic image delivery. Docker/runtime compatibility remains unverified.
Related regression/race tests passed. The real Incus/Btrfs synthetic copy fixture
also passed through the default resolver, proving COW ancestry, independent writes,
source deletion and cleanup. It does not prove image acquisition or runtime use.
PR #482 at `f8517ba` passed all four GHA workflows, including Windows native SSH;
newer changes require their own CI evidence. Private-registry E2E was SKIP because
that job is workflow-dispatch-only.
The requested temporary `docker run --rm`-like Environment flow is scheduled after
VS Code connection acceptance, reusing the existing ephemeral-run implementation.

## Runtime-owned automatic SSH ports

Status: **implemented; installed acceptance pending for this addition**.
`haco env ssh --key <public-key-file> <name>` no longer needs a port argument.
SSH port zero is passed to the Incus runtime, which selects on the Physical Host
and reserves the proxy before guest key changes. Windows GHA at bcc1baf passed this
ordinary default, desktop key/config setup and actual VS Code access.

## Desktop SSH setup and opening

Status: **implemented commands; Windows GHA editor acceptance passed**.
`haco ssh setup [name]` prepares desktop-owned keys and strict host-key pins.
`haco open [--client vscode|ssh] [name]` selects the client; a single Environment
needs no name. Installed GHA covers native SSH, stopped resume and connection reuse,
then actual editor and terminal access at `4f1f512`. See the
[client contract](design/client-adapters-and-vscode-integration.md#desktop-ssh-setup-and-vs-code-opening).

Native Windows fixture checks also cover key/config creation and editor discovery
through the installed trusted Host. A separate development-Ubuntu attempt failed
with PowerShell `exec format error`; a cold raw-Incus fixture failed with the Host
stopped, then passed after normal interactive entry. Those preparation fixtures
alone are not connection acceptance. The local installed version/approval gap and
the exact successful GHA scope are recorded at the top of this document.

Daily CLI inspection is a **partial C6 slice**: `haco env list` shows the registered
name/Workspace/Base; `--json` is available for scripts. List/status output escapes
terminal controls. Broader DNS and connection diagnosis remains incomplete.

## Saved Host customization

Status: **implemented explicit setup/replay; Windows GHA acceptance passed at bcc1baf**.
A user-selected UTF-8 Bash recipe is saved privately by the controller and executed
only in the verified trusted Host. Plain setup replays the snapshot, an explicit
script update replaces it, and clear removes it without execution. No Environment
receives the recipe. Regression coverage exercises private-file/link protections,
serialization, persistence before script failure, replay after service recreation,
target ownership, stdin delivery, and sanitized failure reporting. The Windows GHA
fixture passed ordinary save/replay/update/clear acceptance at bcc1baf (Windows run
34103036390, job 101681633357). Test, Ubuntu and Incus workflows also passed. See
[Host customization](design/trusted-host.md#saved-customization-recipes).

Implicit Host recreation outside controller setup is not yet accepted. Physical
customization and arbitrary package/dotfile recipes have not been run on the user's
installation. The initial source-edit review rejection was resolved by reading the
explicit roadmap C2 requirement and resubmitting the same source-only edit with that
evidence; it is separate from the still-pending local package policy permission.

## Temporary execution

Status: **implemented product CLI; real-Incus acceptance passed at 4adfe19**.
`haco run [--rm] -- <command>` creates an owned temporary Workspace by default,
executes in /workspace and removes the runtime plus its automatic OCI copy.
`--workspace` retains an existing Workspace and Store; `--no-oci` opts out.
Temporary identity is recorded before creation. Canonical deletion checks that
identity under its lifecycle lock, and resource cleanup atomically checks the
Workspace binding. Failed cleanup keeps recovery evidence. Nonzero exits,
cleanup failure and cancellation remain distinct; stdin/TTY is not implemented.

Focused race regressions cover retained-work protection, cleanup failure/recovery,
default source selection, OCI source retention, provider opt-in and literal argv.
Maintained real Incus GHA passed product success, exit 17, retained writes and
cancellation cleanup at 4adfe19 (run 34115004878, job 101719650209). Populated
OCI image execution and local installed acceptance remain unverified. See
[temporary execution](design/temporary-execution.md) and
[ADR 0020](adr/0020-runtime-owned-temporary-workspaces.md).

## Development preview

Status: **partial roadmap C5**. `haco open --port <port> [environment]` adds
loopback HTTP preview with connection reuse and `--close`. Installed Windows HTTP
acceptance and actual browser launch remain pending. See [preview](design/development-preview.md).

## Environment diagnostics

Status: **partial roadmap C6**. `haco doctor [--json] <environment>` reports
Workspace, runtime and sanitized client connection information and checks local
Workspace/DNS/SSH prerequisites without starting or repairing the Environment.
Installed acceptance and external DNS/desktop reachability remain separate.
See [diagnostic scope](design/controller-client-transport.md#environment-diagnostics).

## Saved Policy precedence

Status: **partial roadmap D1**. Matching explicit Policy rules now use
`deny > require-approval > allow`, replacing first-match ordering. Environment
and attribute matching are unchanged; the default remains a fallback. Persistent choices now have typed scope conversion, a protected atomic writer
and audited service integration. Controller stream and terminal component support are implemented; ordinary
product Git queue and notification integration remain unimplemented.
See [precedence](design/policy-and-capability-foundation.md#matching-rule-precedence).

At `a1d084b`, Windows run 34124995997 was cancelled after the job limit.
Installed HTTPS/direct-egress checks passed; the native-access step did not
produce DNS/SSH/VS Code completion markers. This is not acceptance success.
`c05528a` adds bounded child/output waits and progress markers; its actual
Windows run passed DNS/VS Code but failed before setup due to harness CRLF. Three runner regressions passed on local Windows,
including a descendant retaining inherited output.

At `5f824b4`, Ubuntu and Incus passed. Go 1.26/1.27 tests/vet, race and docs passed,
but the test workflow failed: Capability E2E still expected the old approval prompt,
and GoReleaser installation received HTTP 504. The prompt assertion is updated;
local saved-choice/replay/Environment-scope E2E and product CLI E2E passed.
Windows run 34133686648 passed DNS, ordinary SSH reuse/resume and VS Code, then
failed project setup. Production runner decorators now retain the optional stdin
contract for Incus exec; installed acceptance of this fix remains pending.

Saved Policy now also supports Environment/global require-approval. The terminal asks a separate y/N for the current request; saving ask never creates allow. Ordinary Git/notification integration and binding these product flows to immutable Environment identity remain incomplete.

At `347ca50`, test/Ubuntu/Incus workflows passed. Windows run 34135390824 passed DNS, VS Code and project setup save/replay/nonzero/update/clear, then failed the preview server recipe. The fixture now prepares Python if absent and waits for a loopback listener; the previous failure cause is not fully established. Preview/Edge/Environment doctor remain unaccepted. Policy now rechecks one-shot and initially allowed requests immediately before execution; focused race tests passed.

Environment creation now reserves a random instance ID in its canonical lease. Legacy ready state assigns one once under the catalog lock. Saved Policy, approval display and audit can carry that ID; production Git resolves and rechecks it before execution. Same-name recreation cannot inherit an identified saved Environment rule. State/Workspace/Core and Capability/controller/Git race tests passed; ordinary Git saved-choice scope/UI and network identity integration remain partial. See [ADR 0025](adr/0025-environment-approval-identity.md).

At `bffc3fd`, test/Ubuntu/Incus passed. Windows run 34136858725 again passed VS Code and project setup, then failed the preview response assertion because PowerShell received byte content for the extensionless marker. The text/plain fixture correction awaits installed validation. Git pending output now includes the trusted creation identity without new user arguments.

At d4aef8d, all four workflows passed. Windows run [34139245378](https://github.com/SLktEx/Hacocoon/actions/runs/34139245378) verified actual VS Code access, project setup save/replay/nonzero/update/clear, Edge headless preview rendering, HTTP preview reuse/close/refusal and Environment doctor prerequisites. This supersedes the pending preview acceptance above; it does not establish default-browser launch or physical-host acceptance. VPN/NRPT remained SKIP without a VPN/private-name fixture.

The production Capability service now resolves every named request to trusted catalog identity and rechecks it before execution. Environment-scoped saved choices require an identity; no new user argument is needed. Ordinary Git saved-choice scope/UI and real network/provider acceptance remain partial.

At 5272434, GHA Go 1.26/1.27 tests/vet, race, release-config, docs, Ubuntu and Incus passed. The test workflow failed only in the orchestrator E2E, whose approval source name had never been created. The fixture now uses ordinary create/delete, and its local E2E passed. Capability saved-scope/recreation and Git transport-refusal E2Es passed.

The local CI entry point passed docs/workflow checks, then failed because WSL lacks pwsh; subsequent stages in that invocation were unexecuted. Its separate Go stage exposed a test-helper deadlock: an empty select could terminate the SIGKILL helper before the parent checked its live lock. A bounded timer preserves the helper until the parent kills it. The actual subprocess/SIGKILL regression passed 20 repetitions and the run package race tests passed. These are fixture fixes, not changes to cleanup authority.


Real Git push CI is restricted to manual trusted-main dispatch and the fixed SLktEx/Hacocoon-test target. Missing dedicated credentials produce SKIP, not push acceptance. The legacy fixture does not verify installed-product import or interactive approval. See [ADR 0059](adr/0059-dedicated-git-push-test-target.md).

Windows 39b5ce4 reproduced approval setup failure and diagnosed DNS start-limit-hit. Unchanged DNS provisioning now uses idempotent systemd start; changed companion/unit still restart. Installed Windows verification passed at 226991b (run 34479510230). See [service activation](design/name-resolution.md#repeated-setup-and-service-activation).

G1 Windows-file acceptance passed at c4449e1 in [Windows run 34482712957](https://github.com/SLktEx/Hacocoon/actions/runs/34482712957): exclusive copy of a completed Linux bundle, Windows size/hash receipt and Linux import from the existing projected drive. Direct native Windows CLI/export publication is not implemented. See [the route](design/environment-transfer.md#windows-bundle-file-through-existing-drive-projection).

E5 OCI image list/delete now support reviewed --unused candidates (including tagged images without container users). Existing per-image ownership, reference and absence checks remain. Native controller/CLI batch acceptance passed at 9484d06 (run 34493016558); cache/other-resource GC remains planned. See [image selection](design/oci-image-deletion.md#review-unused-image-candidates).

G1 live containerd transfer fixture passed native acceptance at 6974272 (run 34501951826); all applicable CI including Windows passed. It checks an offline image and stopped container writable data after source deletion through both import compositions. Docker/cache/application consistency remain unverified. See [scope](design/environment-transfer.md#live-oci-transfer-acceptance).

G2 now has a read-only native evacuation inventory helper, verified against dedicated WSL Incus. Whole-installation data enumeration, external backup and restored-data comparison remain unimplemented. See [inventory scope](design/environment-transfer.md#evacuation-inventory).

G2 native inventory also reports pool backing references and volume content types without opening them or exposing URI credentials. Eleven focused tests pass; these references do not establish whole-installation coverage or cleanup authority.

G2 adds an optional Linux read-only schema-13 catalog reference projection; no catalog migration, credential output or ownership authority is introduced. Full associations and whole-installation evacuation remain partial.

G2 inventory includes optional separate repository-record and collection-member references. It does not output remote URLs or treat an incomplete/changing directory as a complete backup.
