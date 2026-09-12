# Storage reclamation

[日本語](storage-reclamation.ja.md) | English

Status: **implemented; native Windows/WSL CI acceptance passed**.
This operation recovers unused allocation without deleting objects or shrinking
capacity. Existing-installation and interrupted-worker acceptance remain separate.

When no current operation record exists, `haco reclaim --status` succeeds with an explicit no-saved-result message. This read-only observation creates no operation or registry key and does not prove that reclamation never ran or completed. `--review` has nothing to acknowledge. Malformed records, access failures and missing explicitly requested operation IDs remain errors; pending/failed evidence is never converted to absence.


## Public dispatch and result inspection

From the managed trusted Host, save active work, then run:

```bash
haco reclaim
```

Confirm the stop/restart; `--yes` skips only that prompt. The installed Windows
helper prepares and dispatches one worker. Exit zero means **dispatched**, not
completed. Running sessions disconnect. After restart, enter the Host and run:

```bash
haco reclaim --status
```

There are no required GUID/path/pool arguments. Discovery identifies this installed
controller; it does not itself authorize disk mutation. Missing, invalid or changed
enrollment fails closed. A lost reply can coexist with running work; inspect status
before any new attempt. Pending means unknown completion. Failed status returns
nonzero and preserves evidence. Status never launches, replays or clears work.

A historical installation needs the current normal installer and permanent helper.
No separate Linux-only public reclamation command is provided.

## Reviewing through haco

After inspecting the saved result, explicitly run:

```bash
haco reclaim --review
```

Review freezes the unsuccessful operation and asks for confirmation
(`--yes` skips the prompt). A live continuation, changed operation or malformed
evidence refuses review. It may reopen the enrolled WSL to verify identity, but
does not discard, compact or launch another attempt. A separate `haco reclaim`
is needed to start fresh work after a successful review.

A terminal failure is copied verbatim to `ReviewedFailed-<operation GUID>`,
flushed and read back before replacement is permitted. The old result remains
failed and readable by ID. Interrupted review first retains the exact pending
bytes, then marks the current handoff `interrupted`, preventing a late worker
from executing or publishing its result. Missing archival proof blocks replacement.
Completed/already-interrupted results need no new mutation. Never delete records
to force a retry. Real enrolled interrupted-worker review remains unverified.

## Required result

Reclamation covers the configured Incus Btrfs pool, its loop backing file, outer
WSL ext4 filesystem and the enrolled Windows VHDX. Report filesystem capacity/use,
loop file length/allocation, kernel discard counts and Windows allocation separately.
Missing observations remain unknown; zero reduction is valid. Kernel trimmed bytes
do not measure Windows recovered bytes. Capacity changes, invalid counters,
overflow or filesystem-type changes refuse success.

Workspace/Git, OCI, native volumes and saved snapshots remain. Filesystem references
determine free extents. Do not truncate the backing file, shrink the configured
pool or infer an unused-object deletion list. No hidden backup or data migration
is performed. [Explicit cleanup](../guides/data-lifetime.md) is a separate operation.

## Ownership and Linux stages

Incus owns creation, mounting and pool lifetime. The adapter selects only the
configured, created Btrfs pool at `/var/lib/incus/disks/<pool>.img`, with its
configured mount policy. Custom Incus data layouts are unsupported by this slice.
A cold pool must be mounted through ordinary Incus use, never a second mount owner.

The Linux primitive pins backing-file, mount and loop handles. It rejects symlink
components and hardlinks, verifies single-device Btrfs, one full-file loop association
with matching device/inode and zero offset/size limit, and checks names and native
correspondence before and after mutation. Linux amd64/arm64 needs the required
`openat2` guarantees; there is no weaker fallback. Doctor output alone is not
a mutation precondition.

Both discard stages synchronize first. Outer ext4 FITRIM uses the pinned backing
file's filesystem and requires the installed WSL binding in addition to pool
authority. Other outer filesystems are unsupported. Cancellation inside a kernel
operation retains attempted/result evidence; it does not prove no discard occurred.
Close handles before releasing the ordinary Incus use that kept the pool mounted.

The management-only `storage.reclaim-linux` RPC accepts only canonical
registration/installation IDs, never a pool/path/loop/command. It checks the
root-owned installed identity, trims Btrfs, rechecks identity, then trims ext4.
The five-minute request excludes concurrent calls. Failure skips later stages,
retains cleanup failure separately and returns a failed typed report without raw
subprocess output. Transport loss cannot authorize replay.

## Windows enrollment and file identity

Normal installation captures the exact WSL 2 GUID and random installation ID in
root-owned `/etc/hacocoon/windows-registration.json`. Handle-based traversal,
private atomic no-overwrite publication and serialized same-GUID retries preserve
the original ID. Changed/malformed identity is refused. The separate legacy
distribution-name file remains unchanged.

Windows enrollment binds registration values, Linux installation ID, Windows user
SID and pinned VHDX file identity. It is stored beside the operation record under
the current user's `Software\Hacocoon\Reclamation\<registration GUID>` registry key.
Explicit installer enrollment accepts identical correspondence and never replaces
conflicting enrollment or operation history. A copied Linux record is not authority.

Normal installation verifies and permanently installs the bundled helper under
`%LOCALAPPDATA%/Hacocoon/reclamation/<compact-registration-UUID>/haco-wsl.exe`.
An ownership file identifies that installation. Exclusive staging, checksum, flush
and atomic replacement preserve an executing worker's pinned binary. Unowned,
redirected or conflicting paths are refused; a rerun can finish its own partial
installation. No PATH change, elevation or retained extracted ZIP is required.

The Windows library pins the VHDX and ancestors, refuses reparse points/hardlinks
and holds rename exclusions. It accepts local drive paths, not UNC/device paths,
alternate streams or ambiguous Win32 normalization. Native handle allocation and
file length are distinct from virtual disk capacity. The dynamic, detached VHDX
is opened through its held volume-GUID path without parent disks. File pins remain
held during compaction; do not drop them to work around native open errors.

Sharing violations at OpenVirtualDisk have a 90-second wait budget. Other errors
fail immediately; compaction itself is not retried. Individual synchronous native
calls can outlast the budget. Capacity and virtual identity are rechecked afterward.

## Durable worker sequence

A protected global Windows object keyed by user SID and registration GUID excludes
simultaneous Hacocoon continuations. An existing object or creation error refuses
work without waiting/takeover. It does not prevent the Windows owner from raw WSL
administration; precreation can deny service but cannot grant authority.

Preparation holds exclusion and pins, validates enrollment and installed identity,
and flushes a bounded canonical operation intent before launch. The worker
reacquires/rechecks the exact operation, owner, registration and disk.
Version 2 records Linux-started and its validated report before any shutdown.
Version 1 retains its original disk-only meaning and canonical bytes.
Started pending work is never replayed.

The fixed installed `haco _reclaim-linux` bridge receives only bound IDs.
Its cleared-environment subprocess has a six-minute bound; output is bounded
to 4 KiB and must be a valid typed report. Missing/failed/unknown Linux completion
prevents Windows shutdown. The worker has a ten-minute bound.

The launcher pins its own executable and starts only its fixed worker mode with
explicit Job breakaway, no console, NUL input/error, an OS-selected working directory
and cleared environment. Readiness is exactly `RDY\n` plus EOF, after validation
and before stop. The launcher waits at most three minutes; cancellation closes
only its pipe, never kills/retries the worker. An outer Windows Job can still end
the worker; durable pending evidence remains necessary.

Stop uses fixed `systemctl --no-block poweroff` through System32 WSL's
`--distribution-id <GUID>`. No global shutdown, unregister or name fallback occurs.
Distribution stop does not prove the shared WSL VM released its disk. Do not stop
unrelated distributions or change global WSL idle policy to force readiness.
After any stop attempt, bounded same-GUID resumption is attempted even on failure.
Resuming `/usr/bin/true` alone is not controller readiness.

Final complete/failed results retain each stage. Native API success, process ID,
readiness, and resumed WSL are not substitutes for measured complete reclamation.
Older helpers reject version-2/interrupted state rather than discarding fields.

## Component ownership

| Component | Responsibility |
|---|---|
| `modules/runtime/incus` | Native pool selection, pins, FITRIM and measurements |
| `internal/composition` | Installed identity and configured Linux sequence |
| `internal/reclamation` | Bounded identity/result values; no persisted state |
| `internal/controlapi` | Management transport, exclusion and deadlines |
| `cmd/haco-product`, `internal/reclaimclient` | Confirmation, display and fixed Windows bridge |
| `internal/wslreclaim`, `cmd/haco-wsl` | Enrollment, Windows pins/exclusion, records and worker |

See [ADR 0048](../adr/0048-storage-reclamation-identity.md) for the rejected
path/name-only alternatives and authority rationale.

## Current native acceptance

At `5100d86`, the [Windows user-path job](https://github.com/SLktEx/Hacocoon/actions/runs/34623036552/job/103341362151)
ran public dispatch/status from ordinary Host entry. Allocation fell from
7,964,983,296 to 4,224,712,704 bytes (**3,740,270,592 reclaimed**), with virtual
capacity 1 TiB and Incus pool capacity 128 GiB unchanged. Linux discard, exact-WSL
stop/compact/resume, Host marker and detached Workspace/OCI/snapshot restoration passed.

This is one CI configuration, not every existing installation. Earlier native
Job/open/launch failures remain failures; their causes are not all established.
Windows symlink creation was skipped for privilege in some native runs; junction
refusal passed. Power-loss, cross-user/session, every outer Job and real interrupted
worker review remain unverified. [Acceptance evidence](../status/acceptance-evidence.md#storage)
retains those distinctions and historical receipts.

Repository regressions cover identity replacement, malformed records, ordering,
partial failure, no replay, cancellation, review retention and native refusal.
Dedicated `HACO_E2E_RECLAIM_TRIM=1` tests cover owned Btrfs fixtures;
`HACO_E2E_RECLAIM_OUTER_TRIM=1` separately authorizes their dedicated WSL outer
filesystem. Neither opt-in belongs on an unrelated shared Host.
