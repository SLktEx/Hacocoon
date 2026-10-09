# Btrfs storage layout

Status: **the supported local storage path is Incus-owned loop-backed Btrfs.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage**, **v0.21 Managed Btrfs Transparent Compression**, and **v0.25 Incus-owned Btrfs Storage Acceptance**.

## Default local layout

The runtime accepts only a local `incus_pool` attachment identity. Removed `driver`/`source` attachments are rejected even if a pool exists; failed inspection must not create a replacement. Mount-policy read/readback failures fail closed. Real-Incus storage CI also checks existing rootfs and Workspace sentinel data during pool-policy reconciliation.

The local composition lazily asks Incus to create the default pool without a `source=` override:

```text
Incus pool: haco-local-default
  driver=btrfs
  size=128GiB
  btrfs.mount_options=compress=zstd:3,noatime,nodiscard
        |
        v
/var/lib/incus/disks/haco-local-default.img
  (Incus-owned sparse Linux file)
        |
        v
     loop device
        |
        v
  Btrfs filesystem
        |
        v
/var/lib/incus/storage-pools/haco-local-default
  |- cached Base image volumes
  |- Base builder Environments
  |- trusted haco-host rootfs
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

Incus owns creation of the backing image, loop attachment, Btrfs formatting, mount/unmount lifecycle, and supported loop-pool growth. Hacocoon owns only the desired pool identity and policy. There is no second Host-managed block or mount lifecycle in Hacocoon.

## Sparse file versus WSL sparse VHD

Incus' loop-backed Btrfs pool uses a sparse **Linux file**. The logical 128 GiB pool size is not eagerly allocated in full. This is separate from WSL's `sparseVhd` / sparse-VHDX mode. Hacocoon does not enable WSL sparse-VHD mode as part of this storage design; Windows-host VHDX reclamation is an explicit [reclamation operation](storage-reclamation.md).

## Why rootfs objects share one pool

Base images, builder Environments, trusted-host and ordinary Environment rootfs data share the Hacocoon Btrfs pool so Incus can apply its Btrfs storage-driver behavior across their lifecycle:

- transparent Btrfs compression reduces physical bytes where data is compressible;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- storage maintenance stays scoped to Hacocoon rootfs data rather than arbitrary Host data.

Hacocoon does not create a separate Btrfs filesystem or loop image per Environment merely for isolation. Incus volumes/subvolumes provide logical isolation inside the shared pool.

## Managed mount policy

The desired mount policy is:

```text
compress=zstd:3,noatime,nodiscard
```

`compress=zstd:3` enables transparent compression without `compress-force`; normal Btrfs heuristics may leave incompressible data uncompressed. `noatime` avoids read-triggered access-time metadata writes and unnecessary COW churn. `nodiscard` disables continuous discard so reclamation can remain an explicit batch operation. `autodefrag` is intentionally not enabled because automatic defragmentation can rewrite extents and reduce reflink/COW sharing in a snapshot/clone-heavy rootfs pool.

Mount options mainly affect newly written extents. Hacocoon does not automatically rewrite all existing data merely to recompress it.

## Runtime selection rule

The local composition configures a lazy storage provider. Opening a command that does not need Incus root storage does not create the pool.

Before the first Environment, Base builder or trusted host needs root storage, the provider checks for `haco-local-default`. If it does not exist, Hacocoon asks Incus to create the Btrfs loop pool with the desired size and mount options. Subsequent Hacocoon-owned rootfs operations reuse that pool rather than the Host's unrelated Incus default-profile pool.

When `haco-local-default` already exists, Hacocoon reconciles `btrfs.mount_options` to `compress=zstd:3,noatime,nodiscard` before reuse. The populated pool is not destructively recreated; Incus remains the lifecycle and remount owner.

The runtime expects this Incus-owned pool shape and carries no alternate Host-managed storage ownership path.

## Read-only mount diagnostics

`haco doctor` separates the Incus configuration check (`storage`) from the active filesystem check (`storage_mount`). A configured policy is not evidence that Incus has applied it to the live mount.

| Configured policy | Live observation | Result |
|---|---|---|
| Matches | Verified Btrfs root mount applies the policy | `storage_mount: ok` |
| Matches | Verified Btrfs root mount has different options | `storage_mount: pending`; exit 1 |
| Matches | Missing, malformed, ambiguous or changing identity/mount observation | `storage_mount: failed`; exit 1 |
| Unavailable or differs | Live inspection is skipped | Fix the configuration before interpreting live policy |

The Incus adapter reads the local daemon's pool source and accepts the Incus-owned `disks/<pool>.img` layout. It derives that daemon's pool mountpoint and uses read-only `stat`, `losetup --list --associated` and `findmnt --kernel`. The backing object must be a regular file; its device/inode must match exactly one full-image loop association. The mount must be the root of that Btrfs filesystem and use that loop device. Rechecking the backing identity and association detects observed changes during collection. Equal filenames in different WSL distributions are not sufficient identity.

A live match requires writable Btrfs, `noatime` and `compress=zstd:3`, without active discard, autodefrag or forced compression. The negative `nodiscard` token may be absent in kernel output. The report exposes selected results, not raw paths, mount options or subprocess output. Missing fields, duplicate associations/mounts, truncation, cancellation and uncertain inspection cannot become `ok` or `pending`.

`pending` means the desired configuration is recorded but its live application has not been observed. It does not schedule maintenance or promise that a reboot is safe. Preserve work and arrange an Incus-owned pool remount during maintenance, then diagnose again. Hacocoon never attaches, detaches, remounts or reformats as a diagnostic repair. This point-in-time observation is not an ownership lease or a precondition for subsequent mutations.

The common installer runs this same product doctor before reporting completion. A pending or failed live mount check stops installation with the diagnostic next action; rerunning never bypasses the check.

## Acceptance coverage

Repository CI drives the shipped `haco` and `haco-host` controller clients as an ordinary user against real Incus. It verifies that Incus creates its loop-backed Btrfs pool, the backing image is sparse at the Linux-file level, the configured desired state is `compress=zstd:3,noatime,nodiscard`, and the live filesystem has zstd compression and `noatime` with no active discard mode or autodefrag. It also verifies create/exec/delete/run lifecycle operations reuse the pool and that an old compression-only pool setting is reconciled back to the desired policy.

`findmnt` can omit the negative/default `nodiscard` token. Acceptance therefore requires `nodiscard` in the Incus pool configuration and rejects active `discard` / `discard=async` modes on the live mount.

These checks establish lifecycle and policy behavior on the hosted environment. They do not by themselves establish compression ratio, COW efficiency, Windows-host VHDX compaction effectiveness, or every supported Host configuration.

## Host OCI copy measurement

`TestRealIncusHostToolingE2E` extends the existing standard Host fixture with
byte measurements. It pulls BusyBox, builds and executes a real image with
BuildKit, copies the actual Host OCI area through the canonical resource service,
and reuses the image in an independent networkless receiver. It does not turn the
older synthetic cache/Store fixtures into real-image acceptance.

The fixture samples the Host area before copying, both areas after native copy,
and both areas before/after an 8 MiB random write to a retained container's writable
layer. It then deletes that container and one image tag and samples again while
the Host image and the receiver's second tag remain. Native clone ancestry and
nonzero shared extents in the copied image-content and BuildKit directories are
required. Allocation and exclusive extents must increase after the write; the
Host image must still execute with unchanged content and no copied write.

Each `storage_measurement` record identifies its operation and area. The whole
Store, containerd content blobs and BuildKit directory are reported separately:

| Observation | Command and meaning |
|---|---|
| `logical_bytes` | `du --summarize --apparent-size --block-size=1`: apparent directory contents |
| `allocated_bytes` | `du --summarize --block-size=1`: referenced allocated blocks, including CoW duplicates across copies |
| `extent_total_bytes`, `extent_exclusive_bytes`, `extent_set_shared_bytes` | `btrfs filesystem du --raw --summarize`: FIEMAP extent accounting; set-shared counts overlapping shared extents once within each argument |
| `pool_logical_bytes`, `pool_allocated_bytes` | File size and `du --block-size=1` allocation of this fixture's Incus-owned sparse backing image |

Do not sum per-copy allocation or shared extents to infer unique physical usage.
FIEMAP extent lengths are not a compressed-byte measurement. The pool observation
includes rootfs, metadata and runtime activity; differences are observed allocation
changes, not device write counters or export/import write amplification. Image/tag
absence does not prove physical reclamation, especially while other references
remain. See the [Btrfs command contract](https://btrfs.readthedocs.io/en/latest/btrfs-filesystem.html#subcommand) and the
[versioned summary format](https://github.com/kdave/btrfs-progs/blob/v6.17/cmds/filesystem-du.c#L496-L508).

The observer verifies exact volume ownership and consumers, pauses only those
fixture instances through Incus, synchronizes the filesystem, reads the counters,
and resumes the same instances. All fixture writes use guest/runtime operations;
the observer does not modify backing subvolumes, loop devices or mounts. Failure
retains the printed project/catalog and may retain paused consumers for inspection.
It never adopts an existing installation or deletes an unrelated pool.

Run from the repository root on a dedicated root Linux/WSL Incus/Btrfs host with
normal Host-tool provisioning/network access. The maintained Incus workflow runs
this same test; no separate benchmark workflow is required:

```bash
git rev-parse HEAD
git diff --exit-code
go test -c -o /tmp/haco-host-storage.test ./internal/adapters/incus
sudo env HACO_E2E_HOST_TOOLING=1 /tmp/haco-host-storage.test \
  -test.run='^TestRealIncusHostToolingE2E$' -test.v -test.timeout=25m
```

Keep the full test/CI receipt with its exact tested commit. The fixture additionally
reports OS/kernel, Go, Incus, Btrfs-progs, nerdctl/containerd/BuildKit versions,
native snapshotter and built-image identity. Compile from the recorded clean
checkout; a current worktree revision alone does not authenticate an older binary.
Native measurements must be recorded in [acceptance evidence](../status/acceptance-evidence.md#storage)
before claiming measured acceptance. The following sections define separate
managed-Workspace and rootfs/Image slices. General Base-build workloads,
archive/publish amplification, Docker drivers, last-reference reclamation and
large-workload measurements remain under
[issue #241](https://github.com/SLktEx/Hacocoon/issues/241). Do not generalize these
Btrfs observations to other filesystems or Windows VHDX allocation.

## Managed Workspace lifecycle measurement

The existing required `TestRealIncusSnapshotAggregateE2E` fixture now samples
one managed repository volume through running Snapshot capture, public
`open --new --snapshot`, stopped `env copy`, independent write and deletion.
This is a bounded synthetic Workspace workload; the real OCI workload remains
in the separate Host fixture above. Native acceptance and actual values belong
in [acceptance evidence](../status/acceptance-evidence.md#storage).

The measurement interval starts after the older aggregate fixture's synthetic
Git/rootfs/OCI setup. New payload writes and unlinks use only the exact owned
guest; Snapshot, copy and resource deletion use the existing public lifecycle.
An 8 MiB random file is captured, unlinked from its original Workspace while
saved data remains, restored and copied. A second 8 MiB random file is written
only in the final copy. The fixture then deletes the saved Snapshot and restored
source, samples the surviving copy, and unlinks both payload files there. It
preserves the aggregate fixture's original Git/content/ownership/cleanup checks.

Each named phase reports the whole selected Workspace volume and its fixed
payload directory separately, using the `du` and FIEMAP fields defined above.
Bounded SHA-256 reads connect content to those measurements; identical hashes
alone never establish sharing. Source/saved/restored/copy labels have stable
hashes of their exact project/pool/volume/owner identity. The durable fixture
catalog retains the original ownership records. Capture, restore and copy must
show actual shared extents; the independent write must increase referenced
allocation and exclusive extents without changing the saved or source payload.

Observation holds the canonical Environment-then-Workspace locks, checks native
volume ownership and exclusive attachment, and pauses only the exact running
fixture consumers through Incus. Stopped consumers remain stopped. Identity,
generation, attachment and pause state are checked again before physical reads
and resume. Unexpected state or failed observation retains the fixture, including
paused instances, rather than guessing cleanup. Each sampling phase is bounded
by two minutes and each command by one minute; fixed payload files must be
regular, single-link and exactly 8 MiB. Nothing modifies a backing subvolume,
loop device or mount directly.

Pool counters are marked `pool_scope=whole_shared_pool`. This managed pool also
contains rootfs, Images, metadata and unrelated fixture/Host activity; its deltas
cannot be attributed to the Workspace operation. A single filesystem sync
precedes each sample. [Btrfs documents](https://btrfs.readthedocs.io/en/latest/btrfs-filesystem.html#subcommand)
that this starts deleted-subvolume cleaning but does not wait for completion.
Native object absence therefore proves logical deletion only. Surviving-copy
exclusive/shared transitions and backing allocation after the last live unlink
are observations, with no sleep, retry-to-green or required capacity reduction.
They do not prove completed extent retirement, discard or Windows reclamation.

On a dedicated Linux/WSL Incus Host with the normal managed Btrfs pool and a
cached container image, build from the recorded clean commit and supply the
verified pool name and full image fingerprint:

```bash
git rev-parse HEAD
git diff --exit-code
go build -o /tmp/haco-snapshot-cli ./cmd/haco
go test -c -o /tmp/haco-snapshot-storage.test ./internal/adapters/incus
sudo env HACO_E2E_SNAPSHOT_AGGREGATE=1 \
  HACO_E2E_SNAPSHOT_CLI=/tmp/haco-snapshot-cli \
  HACO_E2E_INCUS_RESUME_POOL=<managed-pool> \
  HACO_E2E_INCUS_RESUME_IMAGE=<full-cached-image-fingerprint> \
  /tmp/haco-snapshot-storage.test \
  -test.run='^TestRealIncusSnapshotAggregateE2E$' -test.v -test.timeout=55m
```

The maintained `incus-snapshots` job already requires this test and supplies the
CLI; no additional workflow or opt-in measurement gate is introduced. The test
prints its private failure-recovery catalog before resource creation and cleans
only exact owned resources on success. Retain the complete receipt and exact
compiled commit. This measures Workspace data, not the separate rootfs/Image
branch below. General Base-build measurement, archive/publish amplification,
completed physical reclamation and larger workloads remain under
[issue #241](https://github.com/SLktEx/Hacocoon/issues/241).

## Rootfs capture and ordinary Image reuse measurement

The same required `TestRealIncusSnapshotAggregateE2E` fixture has a distinct
rootfs observer. It measures a bounded synthetic rootfs workload through two
branches; neither branch infers physical sharing from matching content alone:

1. Before running Snapshot capture, the source guest writes an 8 MiB random
   rootfs payload. The observer samples the source and independent saved rootfs,
   then checks that unlinking the source payload leaves the saved bytes intact.
   Public `open --new --snapshot` copies the saved rootfs directly, and stopped
   `env copy` creates another independent rootfs. Capture, restore and copy must
   show shared payload extents. An 8 MiB copy-only write must increase referenced
   allocation and exclusive extents while saved/restored payloads stay unchanged.
2. The Snapshot's generated ordinary Image is pinned by its full immutable
   fingerprint. Two ordinary `open --new IMAGE` Environments use the existing
   creation controller. Read-only Incus API observations bind the optimized image
   cache to that fingerprint, project and pool; the native cache subvolume must
   be read-only. Cache and Environment payloads must show shared extents. An
   independent 8 MiB write in one Environment must increase its referenced
   allocation and exclusive extents without changing the peer or cached payload.

The saved-rootfs and ordinary-Image branches have separate baselines. Image
publication, export and cache materialization may rewrite the data, so the test
does **not** require or claim sharing from the saved rootfs into the image cache.
The cache is Incus-owned; observation grants no Hacocoon authority to edit it.
This Image was generated by Snapshot capture, not by a general Base-build or
Packer workload. The fixture's earlier synthetic OCI setup is not real OCI
image-content acceptance.

Each fixed phase reports the whole rootfs and its fixed payload directory
separately using the logical, referenced-allocation and FIEMAP fields defined
above. Bounded SHA-256 reads cover only the known 8 MiB regular, single-link
payload files. Area identities are hashes of the verified project, pool and
typed runtime/owner or Image identity; the durable catalog retains exact owned
resources. The ordinary Image fingerprint and origin are recorded explicitly.

Observation holds canonical lifecycle locks, verifies the exact rootfs ownership
and current state, and pauses only running fixture consumers through Incus.
Stopped saved instances remain stopped. A bounded flat kernel mount inventory
in the observer's Host namespace must contain exactly one verified pool mount.
Any submount overlapping a measured rootfs (ancestor, exact path or descendant)
is refused before and after recursive reads, including same-filesystem bind
mounts. Guest device metadata is not evidence of Host mount topology. Rootfs and
parent directory identities are pinned across each observation. Identity and pause state are rechecked
before reads and resume; a failed or ambiguous observation retains the fixture,
including paused consumers, for inspection. Payload writes and unlinks run only
in the owned guest. The observer never writes into backing rootfs/cache
subvolumes or changes loop devices, mounts or production provisioning.

Source deletion and the surviving copy's final payload unlink are logical
deletion observations. A single sync and sample do not wait for completed extent
retirement. Whole-pool allocation remains `pool_scope=whole_shared_pool` and
includes other rootfs, Image, metadata and Host activity. Do not sum per-area
allocation to infer unique storage, attribute pool deltas to one operation, or
interpret FIEMAP lengths as compressed bytes, device writes, publication/export
amplification, discard or Windows VHDX reclamation.

Use the same clean-commit build and aggregate invocation above; there is no new
workflow, provisioning route or measurement flag. Native values and the exact
tested commit/run must be recorded in
[acceptance evidence](../status/acceptance-evidence.md#rootfs-image-sharing)
before claiming this slice passed. Repository checks and prior Workspace/Host
receipts do not establish rootfs/Image acceptance. General Base-build/Packer
efficiency, real OCI workloads, completed physical reclamation, large workloads
and other filesystems remain outside this bounded slice of
[issue #241](https://github.com/SLktEx/Hacocoon/issues/241).

## Workspace boundary

Managed product Workspaces use independent Incus custom volumes in the managed pool, with canonical leases and separate data ownership. Retained external-path Workspaces instead bind an explicitly selected caller-owned directory; they need not live in the pool. Never move arbitrary user source trees into managed storage merely to match the rootfs layout.

## Multiple pools

The default local pool is `haco-local-default`. Future explicitly configured pools may use their own Incus-managed storage identity while preserving the same single-owner lifecycle.
