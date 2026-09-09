# Storage reclamation

[日本語](storage-reclamation.ja.md) | English

Status: **partial, internal implementation**. Pinned Linux identity/allocation,
Btrfs trim and outer ext4 discard are implemented and passed isolated native
acceptance. Trusted target selection, the one-entry workflow and Windows VHDX
compaction/resume are not yet implemented. Windows handle-based file
measurement is implemented internally below. F1 remains incomplete.
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
host merely to pass a test. Windows compaction and the public all-layer workflow
remain unverified. See [ADR 0048](../adr/0048-storage-reclamation-identity.md).

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
8,373,927,936 bytes. No termination, compression or recovered-space claim was made.
Compaction still requires trusted distribution selection, native virtual-disk
identity, safe path-based API opening and stop/compact/resume orchestration. Do not
relax the pinning checks merely to make `OpenVirtualDisk` accept a handle.
