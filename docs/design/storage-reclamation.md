# Storage reclamation

[日本語](storage-reclamation.ja.md) | English

Status: **partial, internal implementation**. Pinned Linux identity/allocation,
Btrfs trim and outer ext4 discard are implemented and passed isolated native
acceptance. Configured Incus pool selection is implemented internally; the public
one-entry workflow is not yet implemented. Windows stop/compact/resume,
handle-based measurement and native compaction are internal implementations
described below. F1 remains incomplete.
This is separate from resource deletion/GC and migration.

## Required result

Reclaim unused allocation through the managed Btrfs pool, Incus loop backing file,
outer WSL filesystem and Windows VHDX. Report each layer's logical size, allocated
bytes before/after, actual reduction and failure/skip status separately. Zero
reduction is valid. A filesystem's reported trimmed byte count is not proof of
Windows space recovery. Do not truncate the backing file or reduce the configured
pool capacity: Incus loop-pool shrink is not the supported path.

Keep Workspace/Git data, OCI content, native volumes and saved snapshots. Kernel
filesystem references determine free extents; reclamation never deletes objects
or guesses which snapshot data is unused. Explicit deletion remains separate.

## Incus and native identity

Incus continues to own pool creation, attachment and mount lifetime. Select only
the configured Hacocoon pool, not a caller-supplied Host path or loop number.
Before mutation, require both trusted pool selection and the live native identity.
The private target primitive alone does not authorize a pool.

The Linux primitive opens backing, mount and loop handles, rejects symlink
components and hardlinks, verifies a single-device Btrfs filesystem, and compares
the loop's backing device/inode and zero offset/size limit. Reopening names detects
observed replacement; retained handles prevent operations from following a later
path substitution. Existing doctor observations are read-only diagnostics and
must not be reused alone as a mutation precondition. Linux amd64/arm64 kernels
without the required openat2 guarantees are unsupported; no weakened fallback.

A cold WSL invocation can see an unmounted pool. Do not mount it independently of
Incus. Normal Host entry or an exactly owned Incus runtime can establish pool use;
close inspection/operation handles before releasing that use. The isolated native
fixture uses a temporary instance and retains the shared image/pool.

## Windows continuation

Planned: a Windows-side continuation must outlive termination of the exact managed
WSL distribution, compact its verified VHDX, report actual Windows allocation and
resume the required entry. Do not terminate every WSL distribution or compact a
whole Windows drive. Preserve step results after failure rather than claiming the
whole chain succeeded. No hidden backup or full Environment recovery is required.

Microsoft provides native [virtual disk compaction](https://learn.microsoft.com/en-us/windows/win32/api/virtdisk/nf-virtdisk-compactvirtualdisk).
Its filesystem-agnostic mode handles zero blocks; actual effectiveness for the
WSL disk must be verified after filesystem discard, not inferred from API success.

## Outer filesystem discard

The ext4 stage uses the already pinned Incus backing-file descriptor. Linux
[ext4 FITRIM](https://github.com/torvalds/linux/blob/v6.6/fs/ext4/ioctl.c)
operates on that file's filesystem; no independent mount path is selected.
Require separate authorization for the managed WSL distribution: permission to
trim one pool alone does not authorize its whole outer filesystem. Other outer
filesystems currently return unsupported. Neither stage is publicly wired yet.

Both stages synchronize before discard, validate native identity before/after,
and retain attempted/kernel-result information if cancellation occurs inside the
kernel operation. Cancellation does not prove that trim was not executed.

## Validation

Sparse allocation, replacement/symlink/hardlink refusal, canceled and invalid
native target regressions passed. The first native inspection failed unsupported
because the pool was not mounted; it performed no trim. Incus-owned mounted
inspection then passed in 22.28s, with exact instance cleanup.

Dedicated WSL Incus/Btrfs inner trim passed in 23.58s, then combined Btrfs/ext4
trim passed in 19.81s. The isolated 1GiB pool's backing allocation went from
72,523,776 bytes before filler removal to 1,417,216 after trim. Retained volume
and native snapshot bytes were unchanged. Logical size stayed 1,073,741,824 bytes.
The ext4 kernel reported 1,075,829,817,344 trimmed bytes; Windows allocation was
not measured. The test removed only its own filler and, after verification, its
exact owned fixture resources. Ownership receipts remain for inspection.

Use `HACO_E2E_RECLAIM_TRIM=1` for the dedicated root Incus fixture. Adding
`HACO_E2E_RECLAIM_OUTER_TRIM=1` separately permits discard of the dedicated
fixture distribution's outer filesystem. Never set this on an unrelated/shared
host merely to pass a test. The public all-layer workflow remains unverified;
separate Windows compaction acceptance is recorded below. See [ADR 0048](../adr/0048-storage-reclamation-identity.md).

## Windows file identity and allocation

Implemented internally in `internal/wslreclaim`, with Windows build constraints:
measurement pins the VHDX and every parent directory, rejects reparse points and
multiple hardlinks, and holds sharing exclusions against rename. Local drive paths
are currently supported; UNC/device paths, alternate streams and ambiguous Win32
normalization are refused. No public caller supplies arbitrary paths today.
This observation alone does not authorize WSL termination or compaction.

Use native handle-based `FILE_STANDARD_INFO` for allocated bytes and file length.
The latter is not the virtual filesystem capacity inside the VHDX. Windows native
acceptance measured a 32MiB sparse fixture at 64KiB allocated, preserved its bytes,
and refused file/parent rename, hardlinks and a junction ancestor. The first
attribute-only open implementation failed the rename test; `GENERIC_READ` fixed
that failure. Native symlink creation was SKIP for missing Windows privilege;
junction coverage passed but does not claim that skipped fixture ran.

The exact dedicated WSL VHDX was read successfully at file length/allocation
8,373,927,936 bytes in the initial measurement-only check. Subsequent compaction
acceptance is below. Public use still requires trusted distribution selection
and stop/compact/resume orchestration. Do not
relax the pinning checks merely to make `OpenVirtualDisk` accept a handle.

## Native compaction acceptance

Internal Windows compaction resolves the held file to a volume-GUID path, opens
VHDX without following parent disks, and requires a dynamic, detached disk.
It retains file/ancestor pins throughout, records native completion separately
from measured allocation, and checks virtual capacity/identifier afterwards.
Cancellation cannot undo a synchronous operation. Sharing violations at native
open have a 90-second retry budget; individual synchronous calls can exceed that
budget. Other errors fail immediately. Compaction itself is never retried.
This remains **partial, internal only**; public orchestration is not implemented.

The first dedicated VHDX attempt FAILED in 4.27s at `OpenVirtualDisk` with a sharing
violation, without attempting compaction. Later isolated native VHDX tests and
real WSL compaction succeeded with the same pins held. The earlier assertion that
pinning conflicts with native open was incorrect; the initial cause is unresolved.
No release/handoff of the file pin is required in the tested native path.

Dedicated WSL compaction PASSED in 17.66s: Windows file length/allocation decreased
from 8,373,927,936 to 6,719,275,008 bytes (1,654,652,928 bytes reclaimed). Virtual
capacity remained 1TiB and its identifier was unchanged. The exact registration
was resumed; the probe hash and nine stopped instance names/states matched.
These observations do not prove every saved Workspace/OCI/snapshot byte.

A subsequent immediate-stop acceptance with bounded open waiting FAILED in
34.21s after 69 open attempts. Sharing violations persisted to the deadline;
compaction was not attempted. Same-registration resumption and the probe/instance
checks passed again. This historical 30-second attempt failed; the later 90-second acceptance below
does not turn it into a pass. Public orchestration remains unfinished.
The probe remains under `/var/lib/haco-reclaim-compact.7NTvx7`.

Native isolated VHDX compaction, retained file identity/rename exclusion,
cancellation, invalid-file refusal and bounded sharing-wait regressions passed.
The public all-layer flow, automatic stop/resume, and full saved-data acceptance
through that flow remain unverified.

## Distribution readiness and registration

On WSL 2.7.13, a stopped distribution listing did not prove native disk readiness:
a read-only open immediately after individual termination FAILED in 4.10s.
A bounded read-only observation subsequently PASSED in 61.62s, with 204 opens
and 57.70s spent waiting; the native disk was detached. Same-registration resume,
probe hash and nine stopped instance records matched. No disk mutation ran.
The 30-second budget was shorter than this observed release time; native open
now allows 90 seconds. This does not guarantee readiness while other WSL users
keep the shared VM alive or when its idle timeout is disabled/longer.

The installed-version [WSL session implementation](https://github.com/microsoft/WSL/blob/2.7.13/src/windows/service/exe/LxssUserSession.cpp)
separates distribution stop from shared-VM idle termination. Do not change global
WSL configuration or stop unrelated distributions to force this step to pass.
The installed CLI has no native compact command; newer upstream source must not
be mistaken for an installed capability.

Internal registration reads use an explicit nonzero GUID under the current user's
WSL registry, literal name/base/VHD filename values and WSL 2. They refuse missing
values, ambiguous paths and default/name fallback. Revalidation rejects changed
registration values. This observation still needs binding to the authorized
managed installation and to held file identity at each mutation boundary; it is
not a new permission source or a completed public continuation.

After this adjustment, exact registration/pinned-file preflight PASSED in 4.42s,
and immediate-stop compaction PASSED in 79.16s with 144 native open attempts.
Windows file length/allocation went from 6,768,558,080 to 6,747,586,560 bytes
(20MiB reclaimed); virtual capacity 1TiB and identifier matched across compaction.
The same registration resumed and the sentinel/nine-instance checks matched.
The acceptance driver performed stop/resume; product orchestration is still planned.
Native registration type/path rejection tests and amd64/arm64 builds passed.
Existing catalog/snapshot data and WSL registration/configuration were not migrated
or rewritten. Full Workspace/OCI/snapshot content acceptance through the future
public all-layer operation is still pending.

## Configured Incus pool entry

`Runtime.PrepareStorageReclamation` now binds trusted local configuration to the
existing pool and held native filesystem/loop/image. It requires one matching
created Btrfs pool, the configured mount policy and the exact installed path
`/var/lib/incus/disks/<pool>.img`. Arbitrary data directories/custom Incus layouts
are currently unsupported; backend output cannot select another Host directory.
It rereads pool correspondence after pinning and before each discard stage.
Missing, malformed, duplicate, truncated, failed or incompatible observations
stop before mutation. Native methods and close are serialized.

This entry never creates a pool or independently mounts it. Normal Incus use must
keep a cold pool mounted before selection. The composition entry supplies
its existing configured pool without accepting caller pool/path arguments. Outer-filesystem authority and
the Windows continuation still need their separate installation binding. The
public all-layer operation remains planned; there is no Linux-only public command.

The existing dedicated Incus/Btrfs fixture now uses this entry and PASSED in
26.52s. An isolated 1GiB pool retained its volume and native snapshot bytes;
backing allocation fell from 72,523,776 to 1,417,216 bytes after filler removal
and inner/outer discard. Incus pool and backing-image absence were checked after
owned fixture cleanup; the ownership receipt remains. Windows allocation was
not measured in this Linux test. Focused selection/refusal tests and race checks
passed. No catalog migration or user command change is introduced by this slice.

## GUID-bound Windows sequence

The installed WSL 2.7.13 [terminate implementation](https://github.com/microsoft/WSL/blob/2.7.13/src/windows/common/WslClient.cpp)
resolves the supplied name again. The internal sequence instead launches a fixed
`systemctl --no-block poweroff` through `wsl.exe --distribution-id <GUID>`.
[WSL systemd](https://learn.microsoft.com/en-us/windows/wsl/systemd) owns shutdown
inside that distribution; no global WSL shutdown, unregister or name fallback is
used. Native read-only acceptance of this stop path passed in 68.82s, then the
same registration resumed with matching sentinel/nine-instance observations.

The Windows binding uses the operating system's System32 `wsl.exe`, a fixed root
working directory and fixed arguments with cleared caller environment. It pins
the VHDX and ancestors across stop, compaction and resume and revalidates registry
values around WSL calls. Request acceptance is not proof of completed shutdown.
Once stop is attempted, failure or cancellation still triggers a bounded two-minute
resume attempt against the same GUID. Compaction/resume failures are preserved
separately; successful resume does not turn failed compaction into success.

This remains an internal sequence. Launching `/usr/bin/true` proves WSL process
resumption only, not Incus/controller readiness. Durable intent/results, exclusive
operation ownership, authorization bound to the installed Physical Host, and a
Windows child that survives its WSL caller are required before public activation.
There is no automatic crash replay or backup in this sequence.

The native internal sequence then PASSED in 193.47s: stop request, 91 native
open attempts, compaction and same-GUID resume. Windows allocation decreased
from 6,883,901,440 to 6,776,946,688 bytes (102MiB), with unchanged 1TiB virtual
capacity and virtual identifier during compaction. The retained sentinel hash
and nine stopped Incus instances matched after resume. This is not full
Workspace/OCI content or controller readiness acceptance. Windows unit/refusal
and failure/cancellation tests passed; amd64/arm64 builds passed. The symlink
fixture was SKIPPED because the Windows account lacked creation privilege;
the junction refusal fixture passed. No catalog or saved-data migration occurs.

## Live continuation exclusion

Before pinning or stopping WSL, the native sequence exclusively creates a
[Windows named mutex object](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createmutexexw)
under the global namespace, keyed by the process user's SID and registration GUID.
Its protected DACL grants the user and SYSTEM access; the handle is not inherited.
An existing object or any creation error refuses the operation. There is no wait,
thread-owned mutex acquisition, automatic retry or takeover. The handle remains
held until resume and disk-pin cleanup finish. Closing it deletes no data.

This only excludes simultaneous Hacocoon continuations. It does not authorize a
registration, prevent its Windows owner from using raw WSL, or preserve an
interrupted operation's result. Precreating the name can cause denial of service,
never permission to proceed. Durable intent/results and interrupted-operation
handling remain required before public activation. Windows object lifetime is
not proof that the prior operation completed successfully.

Native Windows tests passed cross-process contention and reacquisition after
release, separate-GUID independence, invalid identity refusal and idempotent
close. Existing native file/empty-VHD and failure tests passed; amd64/arm64
builds passed. Separate Windows login sessions/users were not exercised, and
symlink creation remained SKIPPED for missing privilege (junction refusal passed).
The dedicated WSL stop/compact/resume acceptance above predates this guard;
that full sequence was not rerun for this change.

## Durable last-operation record

The internal sequence now writes versioned intent under the current Windows
user's `Software\Hacocoon\Reclamation\<registration GUID>` registry key before
requesting shutdown. It holds the continuation guard and records a random
operation ID, registration values and pinned file identity. A single bounded
binary value contains canonical JSON; malformed, unknown-version/field, duplicate
field, oversized or wrong-type values fail closed and remain untouched.
[RegFlushKey](https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regflushkey)
completes persistence before shutdown can start. This flush can affect the user's
registry hive and is limited to intent and final result, not progress polling.

The final record distinguishes complete from failed and retains each stage's
observations, without credentials or subprocess output. Pending/failed records
block another attempt; completed results may be replaced by fresh intent only
for the same registration and disk. There is no automatic replay, acknowledgment
or record deletion. Explicit interrupted-operation review, protected installation
binding and the public workflow are still planned. Existing catalog/snapshot
formats and the name-only Linux interop record are unchanged. The record describes
the stop/compact/resume sequence; later handle-close errors still reach the caller.

At `e263f89`, the dedicated WSL native sequence with both guard and durable
record PASSED in 127.22s (seven native open attempts). Windows allocation fell
from 6,827,278,336 to 6,787,432,448 bytes (38MiB); virtual capacity stayed 1TiB.
The live operation was observed as pending. After process completion, the stored
complete record matched the full operation/registration identifiers, pinned file
identity, measurements and virtual identifier. The same WSL resumed, with matching
sentinel SHA-256 and nine-instance inventory. The completed record remains; no
record or existing data was deleted for acceptance.

This supersedes the untested-sequence qualification for the record/guard above.
It does not establish power-loss recovery, cross-user/session acceptance,
controller readiness, every Workspace/OCI byte, or the future public all-layer
entry. Existing root-owned Linux interop stores a distribution name only; preserve
that format for its current users and add explicit installed-registration/disk
binding before exposing destructive Windows operations.

## Installer registration binding

Normal Windows setup now resolves exactly one literal WSL 2 registration GUID
before common Ubuntu setup, runs common setup against that GUID and rechecks the
name/GUID correspondence afterward. It invokes the installed interop helper by
GUID to capture `/etc/hacocoon/windows-registration.json` as root. The separate
versioned record has a canonical registration GUID and a random installation ID.
Existing `windows-distribution.json` remains a JSON name string for current
connection/notification users. `-SkipIncus` does not enroll a managed Host.

Capture walks directory handles without following symlinks, requires a root-owned
nonwritable parent, serializes callers and publishes a flushed mode-0600 file
without replacing an existing name. Same-GUID retries preserve the installation
ID. Changed GUID, unsafe owner/mode/links, oversized, unknown or malformed records
fail closed and remain for review. A crash can leave an incomplete publication;
there is no automatic recovery or overwrite. Existing installations obtain the
record by rerunning the normal installer; no extra user option is required.

This establishes the Linux side of enrollment, not Windows mutation authority.
The Windows continuation must still bind its pinned VHDX and Windows owner to
this installed Host and enforce that binding. A copied Linux record alone cannot
authorize a replacement registration/disk. Public activation and the independent
Windows child remain pending; no new Core storage interface is introduced.

Fifteen native Linux interop tests passed, including concurrent capture and
unsafe/changed-record refusal. Windows installer component tests and real
registration resolution on the dedicated WSL passed. Actual root-owned record
creation passed. The first repeat/legacy comparison probe FAILED because that
WSL has no `windows-distribution.json`; repeat assertions had not run. Existing
legacy-file content comparison is therefore SKIPPED on this fixture, not counted
as success. Complete installer and Windows mutation-authorization acceptance
remain pending.

A separate native repeat probe then PASSED: root ownership, mode 0600, one link,
and identical bytes/inode after two captures. The absent legacy name record
remained absent. The first probe failure above remains recorded.

## Reading the installed identity from Windows

The installed interop helper now has a read-only registration mode. It uses the
same protected-directory/file and canonical-format checks as capture, but missing
records fail without enrollment and valid records are never rewritten. The
Windows observer launches only this fixed helper mode through the exact GUID,
with the existing cleared execution environment and a two-minute bound. Output
is limited to 4KiB, requires canonical schema/UUIDs and must name that same GUID;
unknown/duplicate fields, extra output, truncation and overflow are rejected.
Raw helper output is not logged and stderr is discarded.

The observer holds the continuation exclusion and VHDX/ancestor handles while
reading, then revalidates native file and registry identity. The result combines
the installed identity, registration, pinned file identity and current Windows
process user SID. It does not persist enrollment, issue a permission or compact
anything. The exclusive observation prevents this query from restarting WSL in
the middle of a Hacocoon compaction. Persisting the authorized correspondence
and enforcing it at mutation remain required before public activation.

Sixteen native Linux interop tests passed, including read-without-enrollment and
unchanged bytes/inode. Windows output/identity/refusal regressions and amd64/arm64
builds passed. Dedicated native observation PASSED in 26.87s with the expected
VHDX file identity and a current Windows owner. The installed Hacocoon helper was
updated on that dedicated WSL; its direct read preserved the record. WSL reported
a root systemd user-session startup warning during helper setup, while the helper
check exited successfully. This is not controller or systemd-user readiness
acceptance. Symlink fixture on Windows remains SKIP for privilege; the Linux
symlink and Windows junction refusals passed. No enrollment/compaction was run by
the observer and full installer/public all-layer acceptance remains pending.

## Persisted enrollment and mutation checks

The Windows internal installer entry now persists schema-1 `Installation` beside
`Operation` in the managed registration's registry key. It holds the existing
exclusion and native file pins while capturing and flushing the exact tuple:
registration values, Linux installation ID, Windows process user SID and VHDX
file identity. Only explicit enrollment creates this value. An identical repeat
is accepted; changed or malformed enrollment is never overwritten. The operation
history is a separate value and is not acknowledged or replaced by enrollment.

The ordinary internal continuation requires this saved binding before writing
operation intent or requesting shutdown. It rejects a different registration,
file or Windows user before querying Linux, then checks the installed identity.
It rereads saved correspondence immediately before stop and compaction, and
revalidates native registration before compaction. It never self-enrolls on a
missing record. Existing operation/catalog/snapshot schemas are unchanged.
A copied Linux identity alone cannot satisfy the Windows file/owner binding.

The internal `haco-wsl.exe enroll <registration-guid>` entry is bundled for Windows
amd64/arm64. Normal managed installation verifies its bundled checksum and invokes
it after Linux identity capture, using the uniquely resolved GUID. It runs as the
Windows caller without elevation and exposes no compaction or general command mode.
SkipIncus remains outside managed enrollment. There is no public
mutation entry or automatic enrollment fallback. Interrupted-operation review,
a Windows child surviving its WSL caller and the public all-layer workflow remain
pending. Raw Windows administration by the owning user is outside this exclusion;
this is not a replacement for controller request authorization.

Native Windows binding/refusal tests passed, including no implicit enrollment,
same-name registration replacement, different installation/user/file, unknown
schema/fields and byte-preserving refusal. amd64/arm64 builds passed. Dedicated
explicit enrollment and unchanged repeat PASSED in 53.48s and retained the prior
operation bytes. The enrolled stop/compact/resume sequence PASSED in 128.69s:
6 native open attempts, allocation 6,840,909,824 to 6,817,841,152 bytes (22MiB),
unchanged 1TiB virtual capacity. Sentinel hash, nine-instance inventory and saved
enrollment bytes matched after resume. Complete public flow/controller/full-data
and power-loss acceptance remain unverified.

At PR #511 head `81d105f`, GHA test (all ten jobs), Ubuntu and Incus workflows
passed; authenticated private-registry acceptance was SKIP behind its gate.
Windows installer/reinstall, Environment egress, native interop/SSH, setup,
preview, doctor and notification checks passed, but the workflow FAILED on
Remote-SSH extension installation and pending-review prerequisite setup-start
(internal). The available log does not establish either root cause. These failures
are not covered by the local reclamation acceptance or counted as success.

## Packaged enrollment acceptance

Status: **partial**. The native helper argument/failure/redaction tests and Windows
PowerShell installer fixtures passed, including duplicate/missing/mismatched
checksums and nonzero helper exit. Installer packaging checks verify architecture
and both executable checksums. Both Windows architecture builds passed. The
existing Windows GHA workflow now runs the native refusal suite before packaging.
The checksum verifies bundled content, not trust in a guest-supplied executable;
only the installer package supplies this fixed entry. No user command or option
is added. Existing installations enroll by rerunning normal managed installation;
changed correspondence remains an explicit refusal, with no automatic migration.

At preceding head `d85df9a`, all four GHA workflows passed. This does not erase the
`81d105f` Windows failures above or establish their causes. At `8c8a543`, test/Ubuntu/Incus GHA passed. Windows native tests, release builds,
package assembly and installer component assertions passed, but the separate BAT
exit test FAILED while deleting its empty fixture directory with a sharing
violation. Fresh install/reinstall and later acceptance were SKIP after that
failure. The test now waits up to ten seconds only for native sharing/lock errors
on the exact empty fixture, with real held-directory release and nonempty-refusal
regressions. This does not retry installation or turn a failed operation into
success. The original CI lock holder is unknown; it is not diagnosed as a product
failure. Full installer GHA on the corrected head remains pending. Public reclamation, independent Windows child and
interrupted-record review remain planned. No Env, Workspace, OCI, snapshot or
catalog schema changes are part of this slice.

The actual installer function invoked the built helper against the dedicated WSL
and PASSED in 47.54s. Existing Installation and Operation bytes were unchanged.
This tests real registration dispatch, not a complete fresh install or compaction.

A dedicated WSL read-only prototype observed its Windows parent in a Job and
successfully launched a detached child with explicit job breakaway and cleared
execution context. The child wrote its isolated marker after the caller exited.
No WSL shutdown or compaction ran. This supports the child-launch mechanism on
this host, not surviving WSL shutdown or public handoff acceptance. Windows
[requires the parent Job to permit breakaway](https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags);
an unsupported Job must fail before shutdown, not silently launch a coupled child.

The first combined local run of the new sharing fixture FAILED because the child
was not yet ready. An explicit ready/release handshake removed that test race;
the same PowerShell 5.1 component-plus-BAT sequence then PASSED. The production
installer is unchanged by this fixture correction.

## Explicit prepared-operation handoff

Status: **partial, internal only**. Preparation validates saved enrollment under
exclusion and native pins, then flushes the existing pending operation record.
Execution takes the exact operation ID, reacquires the same exclusion and pins,
rechecks registration/Windows owner/disk/installed identity, and requires that
same pending record before using the shared stop/compact/resume sequence. Foreign,
stale, completed, failed and malformed records are refused without rewriting.

The synchronous internal entry shares these same authorization and execution
functions. The operation schema and existing data remain unchanged. There is no
implicit adoption, scan/replay, new recovery coordinator or public resume command.
An operation ID selects a prepared request; it is not installation or controller
authority. A preparation/launch crash leaves pending evidence for explicit review.
The internal worker launch/status entry exists below; public all-layer activation
remains planned. Two calls in one process do not prove separate-process handoff.

At `7f4d7f4`, all four GHA workflows PASSED, including the packaged helper
installation/reinstallation and subsequent Windows E2E. This closes that
corrected-head installer acceptance gap; the older failed run remains failed.

Prepared native execution PASSED in 165.68s: 70 open attempts, actual allocation
6,833,569,792 to 6,832,521,216 bytes (1MiB), virtual capacity unchanged at 1TiB.
The saved complete result retained the exact prepared operation/registration/file
identity and matched observed stages. Sentinel hash, instance inventory and saved
enrollment bytes were unchanged. Windows refusal/unit suite and both architecture
builds passed; the Windows symlink fixture was SKIP for privilege. This is a
same-process prepare/reopen/execute test, not an independent worker or complete
public/controller/Workspace/OCI-content acceptance.

## Internal Windows worker and result inspection

Status: **partial; native worker acceptance failed on the local Job context**.
`cmd/haco-wsl` now has internal `_launch`, `_continue` and `_status` modes for an
exact registration and prepared operation ID. These are transport/diagnostic
entries, not new ordinary `haco` commands. No public prepare or all-layer request
is exposed. Worker execution still requires persisted enrollment and the exact
pending target through the shared continuation path.

The launcher pins its own executable and ancestors with the same native file
checks used by disk observation; executable write/delete sharing is excluded,
while the VHDX retains its required write sharing. It starts only its own fixed
worker mode, detached with explicit job breakaway, NUL stdio, OS-sourced working
directory and a cleared environment. No caller path/command or startup retry is
accepted. The PID means dispatched, never completed. The worker currently refuses
any Windows Job or attached console before WSL access. Windows nested outer jobs
can remain after breakaway; support for that case requires a separate decision,
not an implicit fallback. Existing Job restrictions are not modified.

`_status` opens only the existing read-only registry key and checks the exact
operation and registration. It never starts WSL, creates enrollment, acknowledges
records or takes over a worker. Pending omits stage observations because its
outcome is unknown. Complete/failed report only persisted observations. A bootstrap
failure can leave pending after process exit; it is not evidence of success or
that no native action occurred. Interrupted review remains unimplemented.

Windows command/library tests and amd64/arm64 builds passed. The initial shared
file refactor failed the existing nil-disk test; nil-safe refusal was restored and
the full native suite passed. Executable write/rename exclusion and read-only
status byte preservation passed, including the actual retained pending record.
Windows symlink fixture was SKIP for privilege.

The dedicated worker launch FAILED with exit 1. A second explicit diagnostic
launch of the same prepared operation also exited 1 and reported `worker is bound
to a Windows Job` before WSL access. Both processes are terminal and the pending
record is retained. The worker stop/compact/resume path is therefore unverified;
no new pending record was substituted and no saved data was removed. The proposed
narrower Job condition is not applied. Standalone/nested-Job acceptance, startup
error transport, public controller integration and the complete all-layer flow
remain incomplete. Preceding `e7d94d5` passed all four GHA workflows; this new
worker slice requires its own CI evaluation.
